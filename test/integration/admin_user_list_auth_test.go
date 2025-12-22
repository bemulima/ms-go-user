package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/example/user-service/config"
	adminv1 "github.com/example/user-service/internal/adapters/http/admin/v1"
	"github.com/example/user-service/internal/adapters/http/middleware"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/pkg/log"
)

func TestAdminUserListAuthAndRBAC(t *testing.T) {
	users := &userRepoStub{users: map[string]*domain.User{"user-1": {ID: "user-1", Email: "user@example.com"}}}
	verifier := func(ctx context.Context, token string) (string, string, string, error) {
		if token != "valid-token" {
			return "", "", "", errors.New("invalid token")
		}
		return "user-1", "admin", "user@example.com", nil
	}

	cfg := &config.Config{}
	logger := log.New("local")
	rbac := &rbacStub{}
	authMW := middleware.NewAuthMiddlewareWithVerifier(cfg, logger, rbac, users, nil, verifier)
	rbacMW := middleware.NewRBACMiddleware(rbac)

	stub := &manageServiceStub{}
	stub.listUsersFn = func(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
		require.Equal(t, 0, offset)
		require.Equal(t, 10, limit)
		return []domain.User{{ID: "user-1"}}, 1, nil
	}

	e := echo.New()
	group := e.Group("/admin/v1/users", authMW.Handler, rbacMW.RequireAnyRole("admin", "moderator"))
	handler := adminv1.NewHandler(stub)
	handler.RegisterRoutes(group)

	unauthorized := httptest.NewRequest(http.MethodGet, "/admin/v1/users", nil)
	unauthorizedRec := httptest.NewRecorder()
	e.ServeHTTP(unauthorizedRec, unauthorized)
	require.Equal(t, http.StatusUnauthorized, unauthorizedRec.Code)

	authorized := httptest.NewRequest(http.MethodGet, "/admin/v1/users?page=1&per=10", nil)
	authorized.Header.Set(echo.HeaderAuthorization, "Bearer valid-token")
	authorizedRec := httptest.NewRecorder()
	e.ServeHTTP(authorizedRec, authorized)
	require.Equal(t, http.StatusOK, authorizedRec.Code)

	var resp struct {
		Data struct {
			TotalCount int64         `json:"totalCount"`
			Users      []domain.User `json:"users"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(authorizedRec.Body.Bytes(), &resp))
	require.Equal(t, int64(1), resp.Data.TotalCount)
	require.Len(t, resp.Data.Users, 1)
}

type userRepoStub struct {
	users map[string]*domain.User
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

type rbacStub struct{}

func (r *rbacStub) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	return "admin", nil
}

func (r *rbacStub) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (r *rbacStub) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	return false, nil
}

func (r *rbacStub) CheckRole(ctx context.Context, userID, role string) (bool, error) {
	return role == "admin", nil
}

func (r *rbacStub) AssignRole(ctx context.Context, userID, role string) error {
	return nil
}
