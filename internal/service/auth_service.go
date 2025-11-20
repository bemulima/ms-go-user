package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/example/user-service/config"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/events"
	"github.com/example/user-service/internal/ports/broker"
	"github.com/example/user-service/internal/ports/tarantool"
	"github.com/example/user-service/internal/repo"
	pkglog "github.com/example/user-service/pkg/log"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user inactive")
)

type AuthService interface {
	StartSignup(ctx context.Context, traceID, email, password string) (string, error)
	VerifySignup(ctx context.Context, traceID, uuid, code string) (*domain.User, *Tokens, error)
	SignIn(ctx context.Context, traceID, email, password string) (*domain.User, *Tokens, error)
	HandleOAuthCallback(ctx context.Context, traceID, provider string, info OAuthUserInfo) (*domain.User, *Tokens, error)
}

type OAuthUserInfo struct {
	Email       string
	DisplayName *string
	AvatarURL   *string
}

type OAuthProvider string

const (
	ProviderGoogle OAuthProvider = "google"
	ProviderGithub OAuthProvider = "github"
)

type authService struct {
	cfg       *config.Config
	logger    pkglog.Logger
	users     repo.UserRepository
	profiles  repo.UserProfileRepository
	providers repo.UserProviderRepository
	tarantool tarantool.Client
	publisher broker.Publisher
	jwtSigner JWTSigner
	avatars   AvatarIngestor
}

func NewAuthService(
	cfg *config.Config,
	logger pkglog.Logger,
	users repo.UserRepository,
	profiles repo.UserProfileRepository,
	providers repo.UserProviderRepository,
	tarantool tarantool.Client,
	publisher broker.Publisher,
	jwtSigner JWTSigner,
	avatars AvatarIngestor,
) AuthService {
	return &authService{
		cfg:       cfg,
		logger:    logger,
		users:     users,
		profiles:  profiles,
		providers: providers,
		tarantool: tarantool,
		publisher: publisher,
		jwtSigner: jwtSigner,
		avatars:   avatars,
	}
}

type OAuthUserInfo struct {
	ProviderType   string
	ProviderUserID string
	Email          string
	DisplayName    *string
	AvatarURL      *string
	Metadata       map[string]interface{}
}

func (s *authService) StartSignup(ctx context.Context, traceID, email, password string) (string, error) {
	if err := validateEmail(email); err != nil {
		return "", err
	}
	if len(password) < 8 {
		return "", errors.New("password too short")
	}
	uuid, err := s.tarantool.StartRegistration(ctx, email, password)
	if err != nil {
		return "", err
	}
	s.logger.Info().Str("trace_id", traceID).Str("email", email).Msg("signup initiated")
	return uuid, nil
}

func (s *authService) VerifySignup(ctx context.Context, traceID, uuid, code string) (*domain.User, *Tokens, error) {
	result, err := s.tarantool.VerifyRegistration(ctx, uuid, code)
	if err != nil {
		return nil, nil, err
	}
	if err := validateEmail(result.Email); err != nil {
		return nil, nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(result.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}
	existing, err := s.users.FindByEmail(ctx, result.Email)
	if err == nil && existing != nil {
		return nil, nil, fmt.Errorf("user already exists")
	}
	user := &domain.User{Email: strings.ToLower(result.Email), IsActive: true}
	hashStr := string(hash)
	user.SetPasswordHash(hashStr)
	if err := s.users.Create(ctx, user); err != nil {
		return nil, nil, err
	}
	profile := &domain.UserProfile{UserID: user.ID}
	if err := s.profiles.Create(ctx, profile); err != nil {
		return nil, nil, err
	}
	if s.publisher != nil {
		_ = s.publisher.Publish(ctx, "user.created", events.NewUserEvent("user.created", user.ID, user.Email, traceID))
	}
	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, nil, err
	}
	return user, tokens, nil
}

func (s *authService) SignIn(ctx context.Context, traceID, email, password string) (*domain.User, *Tokens, error) {
	user, err := s.users.FindByEmail(ctx, strings.ToLower(email))
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, nil, ErrUserInactive
	}
	if !user.HasPassword() {
		return nil, nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, nil, err
	}
	s.logger.Info().Str("trace_id", traceID).Str("user_id", user.ID).Msg("user signed in")
	return user, tokens, nil
}

func (s *authService) HandleOAuthCallback(ctx context.Context, traceID string, info OAuthUserInfo) (*domain.User, *Tokens, error) {
	if info.ProviderType == "" || info.ProviderUserID == "" {
		return nil, nil, errors.New("provider information required")
	}
	if err := validateEmail(info.Email); err != nil {
		return nil, nil, err
	}

	provider, err := s.providers.FindByProvider(ctx, info.ProviderType, info.ProviderUserID)
	if err == nil && provider != nil {
		user, err := s.users.FindByID(ctx, provider.UserID)
		if err != nil {
			return nil, nil, err
		}
		if !user.IsActive {
			return nil, nil, ErrUserInactive
		}
		tokens, err := s.issueTokens(user)
		if err != nil {
			return nil, nil, err
		}
		s.logger.Info().Str("trace_id", traceID).Str("user_id", user.ID).Msg("oauth user found")
		return user, tokens, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	normalizedEmail := strings.ToLower(info.Email)
	user, err := s.users.FindByEmail(ctx, normalizedEmail)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	if user == nil {
		user = &domain.User{Email: normalizedEmail, IsActive: true}
		if err := s.users.Create(ctx, user); err != nil {
			return nil, nil, err
		}
		profile := &domain.UserProfile{UserID: user.ID, DisplayName: info.DisplayName, AvatarURL: info.AvatarURL}
		if err := s.profiles.Create(ctx, profile); err != nil {
			return nil, nil, err
		}
	}

	if !user.IsActive {
		return nil, nil, ErrUserInactive
	}

	providerRecord := &domain.UserProvider{
		ProviderType:   info.ProviderType,
		ProviderUserID: info.ProviderUserID,
		UserID:         user.ID,
		Metadata:       info.Metadata,
	}
	if err := s.providers.Create(ctx, providerRecord); err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, nil, err
		}

		// Another request linked the provider concurrently; fetch the existing link.
		existingProvider, findErr := s.providers.FindByProvider(ctx, info.ProviderType, info.ProviderUserID)
		if findErr != nil {
			return nil, nil, findErr
		}
		providerRecord = existingProvider
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Info().Str("trace_id", traceID).Str("provider", provider).Str("user_id", user.ID).Msg("oauth callback processed")
	return user, tokens, nil
}

func (s *authService) issueTokens(user *domain.User) (*Tokens, error) {
	access, err := s.jwtSigner.SignAccessToken(user.ID, map[string]interface{}{"email": user.Email}, s.cfg.JWTTTLMinutes)
	if err != nil {
		return nil, err
	}
	refresh, err := s.jwtSigner.SignRefreshToken(user.ID, s.cfg.JWTRefreshTTLMinutes)
	if err != nil {
		return nil, err
	}
	return &Tokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.cfg.JWTTTLMinutes.Seconds()),
	}, nil
}

func validateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid email")
	}
	return nil
}

func (s *authService) googleAuthURL(state string) (string, error) {
	if s.cfg.GoogleClientID == "" || s.cfg.GoogleRedirectURL == "" {
		return "", errors.New("google oauth not configured")
	}
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", s.cfg.GoogleClientID)
	params.Set("redirect_uri", s.cfg.GoogleRedirectURL)
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode(), nil
}

func (s *authService) githubAuthURL(state string) (string, error) {
	if s.cfg.GithubClientID == "" || s.cfg.GithubRedirectURL == "" {
		return "", errors.New("github oauth not configured")
	}
	params := url.Values{}
	params.Set("client_id", s.cfg.GithubClientID)
	params.Set("redirect_uri", s.cfg.GithubRedirectURL)
	params.Set("scope", "read:user user:email")
	params.Set("state", state)
	return "https://github.com/login/oauth/authorize?" + params.Encode(), nil
}

func (s *authService) handleGoogleCallback(ctx context.Context, code string) (*oauthProfile, error) {
	if s.cfg.GoogleClientID == "" || s.cfg.GoogleClientSecret == "" || s.cfg.GoogleRedirectURL == "" {
		return nil, errors.New("google oauth not configured")
	}
	tokenURL := "https://oauth2.googleapis.com/token"
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.cfg.GoogleClientID)
	data.Set("client_secret", s.cfg.GoogleClientSecret)
	data.Set("redirect_uri", s.cfg.GoogleRedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to exchange token: %s", resp.Status)
	}
	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return nil, err
	}
	if tokenRes.AccessToken == "" {
		return nil, errors.New("empty access token from google")
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	userReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)

	userResp, err := s.httpClient.Do(userReq)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()
	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, err
	}
	if userResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user info: %s", userResp.Status)
	}
	var userInfo struct {
		Email      string `json:"email"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Name       string `json:"name"`
		Picture    string `json:"picture"`
	}
	if err := json.Unmarshal(userBody, &userInfo); err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(userInfo.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(userInfo.GivenName + " " + userInfo.FamilyName)
	}
	return &oauthProfile{
		Email:       strings.ToLower(userInfo.Email),
		FirstName:   userInfo.GivenName,
		LastName:    userInfo.FamilyName,
		DisplayName: displayName,
		AvatarURL:   userInfo.Picture,
	}, nil
}

func (s *authService) handleGithubCallback(ctx context.Context, code string) (*oauthProfile, error) {
	if s.cfg.GithubClientID == "" || s.cfg.GithubClientSecret == "" || s.cfg.GithubRedirectURL == "" {
		return nil, errors.New("github oauth not configured")
	}
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", s.cfg.GithubClientID)
	data.Set("client_secret", s.cfg.GithubClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.cfg.GithubRedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to exchange token: %s", resp.Status)
	}
	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return nil, err
	}
	if tokenRes.AccessToken == "" {
		return nil, errors.New("empty access token from github")
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	userReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	userReq.Header.Set("Accept", "application/vnd.github+json")

	userResp, err := s.httpClient.Do(userReq)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()
	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, err
	}
	if userResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user info: %s", userResp.Status)
	}
	var userInfo struct {
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Login     string `json:"login"`
	}
	if err := json.Unmarshal(userBody, &userInfo); err != nil {
		return nil, err
	}
	email := strings.ToLower(userInfo.Email)
	if email == "" {
		email, err = s.fetchGithubPrimaryEmail(ctx, tokenRes.AccessToken)
		if err != nil {
			return nil, err
		}
	}
	displayName := userInfo.Name
	if displayName == "" {
		displayName = userInfo.Login
	}
	return &oauthProfile{
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   userInfo.AvatarURL,
	}, nil
}

func (s *authService) fetchGithubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch github emails: %s", resp.Status)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return strings.ToLower(e.Email), nil
		}
	}
	if len(emails) > 0 {
		return strings.ToLower(emails[0].Email), nil
	}
	return "", errors.New("no email returned from github")
}

func (s *authService) upsertOAuthUser(ctx context.Context, profile *oauthProfile) (*domain.User, error) {
	email := strings.ToLower(profile.Email)
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	var user *domain.User
	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = &domain.User{Email: email, IsActive: true}
			if err := s.users.Create(ctx, user); err != nil {
				return nil, err
			}
			prof := &domain.UserProfile{UserID: user.ID}
			displayName := strings.TrimSpace(profile.DisplayName)
			if displayName != "" {
				prof.DisplayName = &displayName
			}
			if profile.AvatarURL != "" {
				avatar := profile.AvatarURL
				prof.AvatarURL = &avatar
			}
			if err := s.profiles.Create(ctx, prof); err != nil {
				return nil, err
			}
			return user, nil
		}
		return nil, err
	}
	user = existing
	if !user.IsActive {
		user.Activate()
		if err := s.users.Update(ctx, user); err != nil {
			return nil, err
		}
	}

	if user.Profile == nil {
		prof := &domain.UserProfile{UserID: user.ID}
		if err := s.profiles.Create(ctx, prof); err != nil {
			return nil, err
		}
		user.Profile = prof
	}
	displayName := strings.TrimSpace(profile.DisplayName)
	if user.Profile != nil {
		var dnPtr *string
		if displayName != "" {
			dnPtr = &displayName
		}
		var avatarPtr *string
		if profile.AvatarURL != "" {
			avatar := profile.AvatarURL
			avatarPtr = &avatar
		}
		user.Profile.Update(dnPtr, avatarPtr)
		if err := s.profiles.Update(ctx, user.Profile); err != nil {
			return nil, err
		}
	}
	return user, nil
}
