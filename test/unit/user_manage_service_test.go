package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gorm.io/gorm"

	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
)

func TestUserManageService_CreateUser(t *testing.T) {
	t.Parallel()

	users := newManageUserRepo()
	profiles := newManageProfileRepo()
	rbac := &recordingRBAC{}
	display := "Admin User"
	svc := service.NewUserManageService(users, profiles, rbac)

	user, err := svc.CreateUser(context.Background(), service.CreateUserRequest{
		Email:       "Admin@example.com",
		Password:    "Password1",
		DisplayName: &display,
		Role:        "admin",
		Status:      domain.UserStatusBlocked,
	})
	require.NoError(t, err)
	require.Equal(t, "admin@example.com", user.Email)
	require.Equal(t, domain.UserStatusBlocked, user.Status)
	require.False(t, user.IsActive)
	require.NotNil(t, user.Profile)
	require.Equal(t, display, deref(user.Profile.DisplayName))
	require.Equal(t, user.ID, rbac.assignedUserID)
	require.Equal(t, "admin", rbac.assignedRole)
}

func TestUserManageService_UpdateUser(t *testing.T) {
	t.Parallel()

	users := newManageUserRepo()
	profiles := newManageProfileRepo()
	rbac := &recordingRBAC{}
	svc := service.NewUserManageService(users, profiles, rbac)

	original := &domain.User{ID: "user-1", Email: "old@example.com", Status: domain.UserStatusActive, IsActive: true}
	require.NoError(t, users.Create(context.Background(), original))
	require.NoError(t, profiles.Create(context.Background(), &domain.UserProfile{UserID: original.ID, DisplayName: stringPtr("Old")}))

	newEmail := "new@example.com"
	newDisplay := "Updated"
	user, err := svc.UpdateUser(context.Background(), original.ID, service.UpdateUserRequest{
		Email:       &newEmail,
		DisplayName: &newDisplay,
	})
	require.NoError(t, err)
	require.Equal(t, newEmail, user.Email)
	require.NotNil(t, user.Profile)
	require.Equal(t, newDisplay, deref(user.Profile.DisplayName))
}

func TestUserManageService_ChangeStatus(t *testing.T) {
	t.Parallel()

	users := newManageUserRepo()
	profiles := newManageProfileRepo()
	rbac := &recordingRBAC{}
	svc := service.NewUserManageService(users, profiles, rbac)

	u := &domain.User{ID: "user-2", Email: "status@example.com", Status: domain.UserStatusActive, IsActive: true}
	require.NoError(t, users.Create(context.Background(), u))

	updated, err := svc.ChangeStatus(context.Background(), u.ID, domain.UserStatusInactive)
	require.NoError(t, err)
	require.Equal(t, domain.UserStatusInactive, updated.Status)
	require.False(t, updated.IsActive)
}

func TestUserManageService_ChangeRole(t *testing.T) {
	t.Parallel()

	users := newManageUserRepo()
	profiles := newManageProfileRepo()
	rbac := &recordingRBAC{}
	svc := service.NewUserManageService(users, profiles, rbac)

	u := &domain.User{ID: "user-3", Email: "role@example.com", Status: domain.UserStatusActive, IsActive: true}
	require.NoError(t, users.Create(context.Background(), u))

	require.NoError(t, svc.ChangeRole(context.Background(), u.ID, "moderator"))
	require.Equal(t, "moderator", rbac.assignedRole)
	require.Equal(t, u.ID, rbac.assignedUserID)
}

type manageUserRepo struct {
	users  map[string]*domain.User
	lastID int
}

func newManageUserRepo() *manageUserRepo {
	return &manageUserRepo{users: map[string]*domain.User{}}
}

func (r *manageUserRepo) Create(_ context.Context, user *domain.User) error {
	r.lastID++
	if user.ID == "" {
		user.ID = fmt.Sprintf("user-%d", r.lastID)
	}
	r.users[user.ID] = user
	return nil
}

func (r *manageUserRepo) Update(_ context.Context, user *domain.User) error {
	r.users[user.ID] = user
	return nil
}

func (r *manageUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *manageUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *manageUserRepo) Delete(_ context.Context, id string) error {
	delete(r.users, id)
	return nil
}

func (r *manageUserRepo) List(_ context.Context, offset, limit int) ([]domain.User, int64, error) {
	return nil, 0, nil
}

type manageProfileRepo struct {
	profiles map[string]*domain.UserProfile
}

func newManageProfileRepo() *manageProfileRepo {
	return &manageProfileRepo{profiles: map[string]*domain.UserProfile{}}
}

func (r *manageProfileRepo) Create(_ context.Context, profile *domain.UserProfile) error {
	if profile.ID == "" {
		profile.ID = "profile-" + profile.UserID
	}
	r.profiles[profile.UserID] = profile
	return nil
}

func (r *manageProfileRepo) Update(_ context.Context, profile *domain.UserProfile) error {
	r.profiles[profile.UserID] = profile
	return nil
}

func (r *manageProfileRepo) FindByUserID(_ context.Context, userID string) (*domain.UserProfile, error) {
	if profile, ok := r.profiles[userID]; ok {
		return profile, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type recordingRBAC struct {
	assignedUserID string
	assignedRole   string
}

func (r *recordingRBAC) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	return "", nil
}

func (r *recordingRBAC) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (r *recordingRBAC) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	return false, nil
}

func (r *recordingRBAC) CheckRole(ctx context.Context, userID, role string) (bool, error) {
	return false, nil
}

func (r *recordingRBAC) AssignRole(ctx context.Context, userID, role string) error {
	r.assignedUserID = userID
	r.assignedRole = role
	return nil
}

func stringPtr(value string) *string {
	return &value
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
