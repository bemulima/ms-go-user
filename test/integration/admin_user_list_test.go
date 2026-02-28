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

	adminv1 "github.com/example/user-service/internal/adapters/http/admin/v1"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
)

func TestAdminUserListRoute(t *testing.T) {
	stub := &manageServiceStub{}
	stub.listUsersFn = func(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
		require.Equal(t, 20, offset)
		require.Equal(t, 10, limit)
		return []domain.User{{ID: "user-1"}, {ID: "user-2"}}, 42, nil
	}

	e := echo.New()
	handler := adminv1.NewHandler(stub, nil)
	group := e.Group("/admin/v1/users")
	handler.RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users?page=3&per=10", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data struct {
			TotalCount int64         `json:"totalCount"`
			Users      []domain.User `json:"users"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, int64(42), resp.Data.TotalCount)
	require.Len(t, resp.Data.Users, 2)
	require.Equal(t, "user-1", resp.Data.Users[0].ID)
}

type manageServiceStub struct {
	listUsersFn func(ctx context.Context, offset, limit int) ([]domain.User, int64, error)
}

func (s *manageServiceStub) CreateUser(ctx context.Context, req service.CreateUserRequest) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (s *manageServiceStub) UpdateUser(ctx context.Context, userID string, req service.UpdateUserRequest) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (s *manageServiceStub) ChangeStatus(ctx context.Context, userID string, status domain.UserStatus) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (s *manageServiceStub) ChangeRole(ctx context.Context, userID, role string) error {
	return errors.New("not implemented")
}

func (s *manageServiceStub) ListUsers(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	if s.listUsersFn != nil {
		return s.listUsersFn(ctx, offset, limit)
	}
	return nil, 0, nil
}
