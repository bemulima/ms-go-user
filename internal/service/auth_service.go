package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

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

type authService struct {
	cfg       *config.Config
	logger    pkglog.Logger
	users     repo.UserRepository
	profiles  repo.UserProfileRepository
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
		tarantool: tarantool,
		publisher: publisher,
		jwtSigner: jwtSigner,
		avatars:   avatars,
	}
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

func (s *authService) HandleOAuthCallback(ctx context.Context, traceID, provider string, info OAuthUserInfo) (*domain.User, *Tokens, error) {
	email := strings.ToLower(info.Email)
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		user = &domain.User{Email: email, IsActive: true}
		if err := s.users.Create(ctx, user); err != nil {
			return nil, nil, err
		}
		if s.publisher != nil {
			_ = s.publisher.Publish(ctx, "user.created", events.NewUserEvent("user.created", user.ID, user.Email, traceID))
		}
	}

	profile, err := s.profiles.FindByUserID(ctx, user.ID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		profile = &domain.UserProfile{UserID: user.ID}
		if err := s.profiles.Create(ctx, profile); err != nil {
			return nil, nil, err
		}
	}

	var avatarURL *string
	if info.AvatarURL != nil && *info.AvatarURL != "" && s.avatars != nil {
		stored, ingestErr := s.avatars.Ingest(ctx, traceID, *info.AvatarURL)
		if ingestErr != nil {
			s.logger.Warn().Str("trace_id", traceID).Str("provider", provider).Str("avatar_url", *info.AvatarURL).Err(ingestErr).Msg("failed to ingest avatar")
		} else {
			avatarURL = &stored
		}
	}

	profile.Update(info.DisplayName, avatarURL)
	if err := s.profiles.Update(ctx, profile); err != nil {
		return nil, nil, err
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
