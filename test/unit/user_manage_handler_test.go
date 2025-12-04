package unit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	adminv1 "github.com/example/user-service/internal/adapters/http/admin/v1"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
)

func TestUserManageHandler_CreateUser(t *testing.T) {
	t.Parallel()

	mockSvc := &mockManageService{
		createUserFn: func(ctx context.Context, req service.CreateUserRequest) (*domain.User, error) {
			require.Equal(t, domain.UserStatusBlocked, req.Status)
			return &domain.User{
				ID:     "user-1",
				Email:  strings.ToLower(req.Email),
				Status: req.Status,
				Profile: &domain.UserProfile{
					UserID: "user-1",
				},
			}, nil
		},
	}
	handler := adminv1.NewHandler(mockSvc)

	e := echo.New()
	body := `{"email":"Admin@example.com","password":"Password1","role":"admin","status":"blocked"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.CreateUser(c))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data domain.User `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "user-1", resp.Data.ID)
	require.Equal(t, domain.UserStatusBlocked, resp.Data.Status)
}

func TestUserManageHandler_UpdateUser_NotFound(t *testing.T) {
	t.Parallel()

	mockSvc := &mockManageService{
		updateUserFn: func(ctx context.Context, userID string, req service.UpdateUserRequest) (*domain.User, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	handler := adminv1.NewHandler(mockSvc)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/123", strings.NewReader(`{"email":"x@example.com"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("123")

	require.NoError(t, handler.UpdateUser(c))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUserManageHandler_ChangeStatus(t *testing.T) {
	t.Parallel()

	mockSvc := &mockManageService{
		changeStatusFn: func(ctx context.Context, userID string, status domain.UserStatus) (*domain.User, error) {
			return &domain.User{ID: userID, Status: status, Profile: &domain.UserProfile{UserID: userID}}, nil
		},
	}
	handler := adminv1.NewHandler(mockSvc)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/42/status", strings.NewReader(`{"status":"active"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("42")

	require.NoError(t, handler.ChangeStatus(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data domain.User `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, domain.UserStatusActive, resp.Data.Status)
}

func TestUserManageHandler_ChangeRole_Error(t *testing.T) {
	t.Parallel()

	mockSvc := &mockManageService{
		changeRoleFn: func(ctx context.Context, userID, role string) error {
			return errors.New("bad role")
		},
	}
	handler := adminv1.NewHandler(mockSvc)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/99/role", strings.NewReader(`{"role":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("99")

	require.NoError(t, handler.ChangeRole(c))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

type mockManageService struct {
	createUserFn   func(ctx context.Context, req service.CreateUserRequest) (*domain.User, error)
	updateUserFn   func(ctx context.Context, userID string, req service.UpdateUserRequest) (*domain.User, error)
	changeStatusFn func(ctx context.Context, userID string, status domain.UserStatus) (*domain.User, error)
	changeRoleFn   func(ctx context.Context, userID, role string) error
}

func (m *mockManageService) CreateUser(ctx context.Context, req service.CreateUserRequest) (*domain.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, req)
	}
	return nil, nil
}

func (m *mockManageService) UpdateUser(ctx context.Context, userID string, req service.UpdateUserRequest) (*domain.User, error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, userID, req)
	}
	return nil, nil
}

func (m *mockManageService) ChangeStatus(ctx context.Context, userID string, status domain.UserStatus) (*domain.User, error) {
	if m.changeStatusFn != nil {
		return m.changeStatusFn(ctx, userID, status)
	}
	return nil, nil
}

func (m *mockManageService) ChangeRole(ctx context.Context, userID, role string) error {
	if m.changeRoleFn != nil {
		return m.changeRoleFn(ctx, userID, role)
	}
	return nil
}
