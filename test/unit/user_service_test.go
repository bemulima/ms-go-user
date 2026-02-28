package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestUserService_UpdateProfile(t *testing.T) {
	users := newUserRepoStub()
	profiles := newProfileRepoStub()
	svc := service.NewUserService(users, profiles, identityRepoStub{})
	display := "New Name"

	profile, err := svc.UpdateProfile(context.Background(), "user-1", &display)
	require.NoError(t, err)
	assert.Equal(t, &display, profile.DisplayName)
	assert.Nil(t, profile.AvatarFileID)
}

func TestUserService_SetAvatarFileID(t *testing.T) {
	users := newUserRepoStub()
	profiles := newProfileRepoStub()
	svc := service.NewUserService(users, profiles, identityRepoStub{})

	profile, err := svc.SetAvatarFileID(context.Background(), "user-1", "file-123")
	require.NoError(t, err)
	require.NotNil(t, profile.AvatarFileID)
	assert.Equal(t, "file-123", *profile.AvatarFileID)
}
