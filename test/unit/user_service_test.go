package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/example/user-service/internal/adapters/tarantool"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
)

type userRepoStub struct {
	users map[string]*domain.User
}

func newUserRepoStub() *userRepoStub {
	return &userRepoStub{users: map[string]*domain.User{"user-1": {ID: "user-1", Email: "user@example.com"}}}
}

func (r *userRepoStub) Create(ctx context.Context, user *domain.User) error { return nil }
func (r *userRepoStub) Update(ctx context.Context, user *domain.User) error {
	r.users[user.ID] = user
	return nil
}
func (r *userRepoStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, errors.New("not found")
}
func (r *userRepoStub) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return nil, errors.New("not found")
}
func (r *userRepoStub) Delete(ctx context.Context, id string) error { return nil }
func (r *userRepoStub) List(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	return nil, 0, nil
}

type profileRepoStub struct {
	profiles map[string]*domain.UserProfile
}

func newProfileRepoStub() *profileRepoStub {
	return &profileRepoStub{profiles: map[string]*domain.UserProfile{"user-1": {ID: "profile-1", UserID: "user-1"}}}
}

func (p *profileRepoStub) Create(ctx context.Context, profile *domain.UserProfile) error { return nil }
func (p *profileRepoStub) Update(ctx context.Context, profile *domain.UserProfile) error {
	p.profiles[profile.UserID] = profile
	return nil
}
func (p *profileRepoStub) FindByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	if profile, ok := p.profiles[userID]; ok {
		return profile, nil
	}
	return nil, errors.New("not found")
}

type identityRepoStub struct{}

func (identityRepoStub) Create(ctx context.Context, identity *domain.UserIdentity) error { return nil }
func (identityRepoStub) FindByProviderUserID(ctx context.Context, provider domain.IdentityProvider, providerUserID string) (*domain.UserIdentity, error) {
	return nil, errors.New("not found")
}
func (identityRepoStub) FindByUserAndProvider(ctx context.Context, userID string, provider domain.IdentityProvider) (*domain.UserIdentity, error) {
	return nil, errors.New("not found")
}
func (identityRepoStub) Delete(ctx context.Context, identity *domain.UserIdentity) error { return nil }

type tarantoolStub struct{}

func (tarantoolStub) StartRegistration(ctx context.Context, email, password string) (string, error) {
	return "", nil
}
func (tarantoolStub) VerifyRegistration(ctx context.Context, uuid, code string) (*tarantool.VerificationResult, error) {
	return nil, nil
}
func (tarantoolStub) StartEmailChange(ctx context.Context, userID, email string) (string, error) {
	return "uuid-change", nil
}
func (tarantoolStub) VerifyEmailChange(ctx context.Context, uuid, code string) (*tarantool.VerificationResult, error) {
	return &tarantool.VerificationResult{Email: "new@example.com"}, nil
}

func TestUserService_UpdateProfile(t *testing.T) {
	users := newUserRepoStub()
	profiles := newProfileRepoStub()
	svc := service.NewUserService(users, profiles, identityRepoStub{}, tarantoolStub{})
	display := "New Name"
	avatar := "http://avatar"

	profile, err := svc.UpdateProfile(context.Background(), "user-1", &display, &avatar)
	require.NoError(t, err)
	assert.Equal(t, &display, profile.DisplayName)
	assert.Equal(t, &avatar, profile.AvatarURL)
}

func TestUserService_VerifyEmailChange(t *testing.T) {
	users := newUserRepoStub()
	profiles := newProfileRepoStub()
	svc := service.NewUserService(users, profiles, identityRepoStub{}, tarantoolStub{})

	user, err := svc.VerifyEmailChange(context.Background(), "user-1", "uuid", "code")
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", user.Email)
}
