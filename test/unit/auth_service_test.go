package unit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/example/user-service/config"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/ports/tarantool"
	"github.com/example/user-service/internal/service"
	pkglog "github.com/example/user-service/pkg/log"
)

type fakeUserRepo struct {
	users map[string]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]*domain.User{}}
}

func (f *fakeUserRepo) Create(ctx context.Context, user *domain.User) error {
	user.ID = "user-1"
	f.users[strings.ToLower(user.Email)] = user
	return nil
}

func (f *fakeUserRepo) Update(ctx context.Context, user *domain.User) error {
	f.users[strings.ToLower(user.Email)] = user
	return nil
}

func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if user, ok := f.users[strings.ToLower(email)]; ok {
		return user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeUserRepo) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeUserRepo) List(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	return nil, 0, nil
}

type fakeProfileRepo struct {
	profiles map[string]*domain.UserProfile
}

func newFakeProfileRepo() *fakeProfileRepo {
	return &fakeProfileRepo{profiles: map[string]*domain.UserProfile{}}
}

func (f *fakeProfileRepo) Create(ctx context.Context, profile *domain.UserProfile) error {
	profile.ID = "profile-1"
	f.profiles[profile.UserID] = profile
	return nil
}

func (f *fakeProfileRepo) Update(ctx context.Context, profile *domain.UserProfile) error {
	f.profiles[profile.UserID] = profile
	return nil
}

func (f *fakeProfileRepo) FindByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	if profile, ok := f.profiles[userID]; ok {
		return profile, nil
	}
	return nil, errors.New("not found")
}

type fakeProviderRepo struct {
	providers map[string]*domain.UserProvider
}

func newFakeProviderRepo() *fakeProviderRepo {
	return &fakeProviderRepo{providers: map[string]*domain.UserProvider{}}
}

func (f *fakeProviderRepo) key(providerType, providerUserID string) string {
	return providerType + ":" + providerUserID
}

func (f *fakeProviderRepo) Create(ctx context.Context, provider *domain.UserProvider) error {
	provider.ID = "provider-" + provider.ProviderUserID
	f.providers[f.key(provider.ProviderType, provider.ProviderUserID)] = provider
	return nil
}

func (f *fakeProviderRepo) Update(ctx context.Context, provider *domain.UserProvider) error {
	f.providers[f.key(provider.ProviderType, provider.ProviderUserID)] = provider
	return nil
}

func (f *fakeProviderRepo) Delete(ctx context.Context, id string) error { return nil }

func (f *fakeProviderRepo) FindByProvider(ctx context.Context, providerType, providerUserID string) (*domain.UserProvider, error) {
	if provider, ok := f.providers[f.key(providerType, providerUserID)]; ok {
		return provider, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeProviderRepo) FindByUserID(ctx context.Context, userID string) ([]domain.UserProvider, error) {
	var result []domain.UserProvider
	for _, provider := range f.providers {
		if provider.UserID == userID {
			result = append(result, *provider)
		}
	}
	return result, nil
}

type fakeRBACClient struct {
	assignments map[string]string
}

func newFakeRBACClient() *fakeRBACClient {
	return &fakeRBACClient{assignments: map[string]string{}}
}

func (f *fakeRBACClient) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	if f.assignments == nil {
		return "user", nil
	}
	if role, ok := f.assignments[userID]; ok {
		return role, nil
	}
	return "user", nil
}

func (f *fakeRBACClient) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (f *fakeRBACClient) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	return false, nil
}

func (f *fakeRBACClient) CheckRole(ctx context.Context, userID, role string) (bool, error) {
	return false, nil
}

func (f *fakeRBACClient) AssignRole(ctx context.Context, userID, role string) error {
	if f.assignments == nil {
		f.assignments = map[string]string{}
	}
	f.assignments[userID] = role
	return nil
}

type recordingRBACClient struct {
	assignCalled bool
	assignedUser string
	assignedRole string
	getRoleCalls []string
	roleByUser   map[string]string
}

func (r *recordingRBACClient) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	r.getRoleCalls = append(r.getRoleCalls, userID)
	if r.roleByUser != nil {
		if role, ok := r.roleByUser[userID]; ok {
			return role, nil
		}
	}
	return "", nil
}

func (r *recordingRBACClient) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (r *recordingRBACClient) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	return false, nil
}

func (r *recordingRBACClient) CheckRole(ctx context.Context, userID, role string) (bool, error) {
	return false, nil
}

func (r *recordingRBACClient) AssignRole(ctx context.Context, userID, role string) error {
	r.assignCalled = true
	r.assignedUser = userID
	r.assignedRole = role
	return nil
}

type recordingJWTSigner struct {
	claims map[string]interface{}
}

func (r *recordingJWTSigner) SignAccessToken(subject string, claims map[string]interface{}, ttl time.Duration) (string, error) {
	if claims == nil {
		return "", errors.New("claims required")
	}
	r.claims = make(map[string]interface{}, len(claims))
	for k, v := range claims {
		r.claims[k] = v
	}
	return "access-token", nil
}

func (r *recordingJWTSigner) SignRefreshToken(subject string, ttl time.Duration) (string, error) {
	return "refresh-token", nil
}

type fakeTarantool struct {
	email    string
	password string
}

func (f *fakeTarantool) StartRegistration(ctx context.Context, email, password string) (string, error) {
	f.email = email
	f.password = password
	return "uuid-1", nil
}

func (f *fakeTarantool) VerifyRegistration(ctx context.Context, uuid, code string) (*tarantool.VerificationResult, error) {
	return &tarantool.VerificationResult{Email: f.email, Password: f.password}, nil
}

func (f *fakeTarantool) StartEmailChange(ctx context.Context, userID, email string) (string, error) {
	return "uuid-change", nil
}

func (f *fakeTarantool) VerifyEmailChange(ctx context.Context, uuid, code string) (*tarantool.VerificationResult, error) {
	return &tarantool.VerificationResult{Email: "new@example.com"}, nil
}

type fakePublisher struct{}

func (fakePublisher) Publish(ctx context.Context, routingKey string, payload interface{}) error {
	return nil
}
func (fakePublisher) Close() error { return nil }

type fakeAvatarIngestor struct{}

func (fakeAvatarIngestor) Ingest(ctx context.Context, traceID, ownerID, avatarURL string) (string, error) {
	return avatarURL, nil
}

func TestAuthService_StartSignup(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)
	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	uuid, err := auth.StartSignup(context.Background(), "trace-1", "user@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, "uuid-1", uuid)
}

func TestAuthService_StartSignup_InvalidPassword(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)
	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	_, err = auth.StartSignup(context.Background(), "trace-1", "user@example.com", "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "digit")
}

func TestAuthService_StartSignup_DuplicateEmail(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)
	users := newFakeUserRepo()
	users.users[strings.ToLower("user@example.com")] = &domain.User{ID: "user-1", Email: "user@example.com"}
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	_, err = auth.StartSignup(context.Background(), "trace-1", "user@example.com", "password123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user already exists")
}

func TestAuthService_VerifySignup(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)
	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{email: "USER@EXAMPLE.COM", password: "password123"}
	rbacClient := newFakeRBACClient()
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, rbacClient, fakePublisher{}, signer, fakeAvatarIngestor{})

	user, tokens, err := auth.VerifySignup(context.Background(), "trace-1", "uuid-1", "0000")
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotNil(t, tokens)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotNil(t, user.PasswordHash)
	assert.Equal(t, "user@example.com", user.Email)
	assert.Equal(t, "user", rbacClient.assignments[user.ID])
}

func TestAuthService_VerifySignup_AssignsDefaultRoleAndResolvesUserRole(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	jwtSigner := &recordingJWTSigner{}
	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{email: "user@example.com", password: "password123"}
	expectedRole := "member"
	rbacClient := &recordingRBACClient{roleByUser: map[string]string{"user-1": expectedRole}}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, rbacClient, fakePublisher{}, jwtSigner, fakeAvatarIngestor{})

	user, tokens, err := auth.VerifySignup(context.Background(), "trace-1", "uuid-1", "0000")
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, tokens)
	require.NotNil(t, jwtSigner.claims)

	assert.True(t, rbacClient.assignCalled)
	assert.Equal(t, user.ID, rbacClient.assignedUser)
	assert.Equal(t, "user", rbacClient.assignedRole)
	assert.Equal(t, []string{user.ID}, rbacClient.getRoleCalls)
	assert.Equal(t, expectedRole, jwtSigner.claims["role"])
	assert.Equal(t, "access-token", tokens.AccessToken)
	assert.Equal(t, "refresh-token", tokens.RefreshToken)
}

func TestAuthService_VerifySignup_InvalidCode(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)
	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{email: "user@example.com", password: "password123"}
	rbacClient := newFakeRBACClient()
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, rbacClient, fakePublisher{}, signer, fakeAvatarIngestor{})

	_, _, err = auth.VerifySignup(context.Background(), "trace-1", "uuid-1", "12a4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification code")
}

func TestAuthService_HandleOAuthCallback_CreateAndLink(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)
	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	displayName := "OAuth User"
	user, tokens, err := auth.HandleOAuthCallback(context.Background(), "trace-1", "google", service.OAuthUserInfo{
		ProviderType:   "google",
		ProviderUserID: "oauth-1",
		Email:          "oauth@example.com",
		DisplayName:    &displayName,
		Metadata:       map[string]interface{}{"locale": "en"},
	})

	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotNil(t, tokens)
	assert.NotEmpty(t, tokens.AccessToken)

	provider, err := providers.FindByProvider(context.Background(), "google", "oauth-1")
	require.NoError(t, err)
	assert.Equal(t, user.ID, provider.UserID)
}

func TestAuthService_HandleOAuthCallback_ExistingProvider(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)

	users := newFakeUserRepo()
	existingUser := &domain.User{ID: "user-42", Email: "linked@example.com", IsActive: true}
	users.users[strings.ToLower(existingUser.Email)] = existingUser

	providers := newFakeProviderRepo()
	providers.providers[providers.key("google", "oauth-1")] = &domain.UserProvider{ProviderType: "google", ProviderUserID: "oauth-1", UserID: existingUser.ID}

	tarantoolClient := &fakeTarantool{}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, newFakeProfileRepo(), providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	user, tokens, err := auth.HandleOAuthCallback(context.Background(), "trace-1", "google", service.OAuthUserInfo{
		ProviderType:   "google",
		ProviderUserID: "oauth-1",
		Email:          existingUser.Email,
	})

	require.NoError(t, err)
	assert.Equal(t, existingUser.ID, user.ID)
	assert.NotNil(t, tokens)
	assert.Equal(t, 1, len(providers.providers))
}

func TestAuthService_HandleOAuthCallback_InactiveUser(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)

	users := newFakeUserRepo()
	inactiveUser := &domain.User{ID: "user-99", Email: "inactive@example.com", IsActive: false}
	users.users[strings.ToLower(inactiveUser.Email)] = inactiveUser

	providers := newFakeProviderRepo()
	providers.providers[providers.key("google", "inactive-1")] = &domain.UserProvider{ProviderType: "google", ProviderUserID: "inactive-1", UserID: inactiveUser.ID}

	tarantoolClient := &fakeTarantool{}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, newFakeProfileRepo(), providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	user, tokens, err := auth.HandleOAuthCallback(context.Background(), "trace-1", "google", service.OAuthUserInfo{
		ProviderType:   "google",
		ProviderUserID: "inactive-1",
		Email:          inactiveUser.Email,
	})

	assert.ErrorIs(t, err, service.ErrUserInactive)
	assert.Nil(t, user)
	assert.Nil(t, tokens)
}

func TestAuthService_SignIn_ResolvesRoleAndSetsJWT(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret", JWTTTLMinutes: time.Minute, JWTRefreshTTLMinutes: time.Hour}
	jwtSigner := &recordingJWTSigner{}

	hash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	require.NoError(t, err)
	pw := string(hash)

	users := newFakeUserRepo()
	users.users["user@example.com"] = &domain.User{
		ID:           "user-1",
		Email:        "user@example.com",
		PasswordHash: &pw,
		IsActive:     true,
	}

	rbacClient := &recordingRBACClient{roleByUser: map[string]string{"user-1": "admin"}}

	auth := service.NewAuthService(cfg, pkglog.New("test"), users, newFakeProfileRepo(), newFakeProviderRepo(), &fakeTarantool{}, rbacClient, fakePublisher{}, jwtSigner, fakeAvatarIngestor{})

	user, tokens, err := auth.SignIn(context.Background(), "trace-1", "user@example.com", "Password123!")
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, tokens)

	assert.Equal(t, []string{"user-1"}, rbacClient.getRoleCalls)
	require.NotNil(t, jwtSigner.claims)
	assert.Equal(t, "admin", jwtSigner.claims["role"])
	assert.Equal(t, "access-token", tokens.AccessToken)
	assert.Equal(t, "refresh-token", tokens.RefreshToken)
}
