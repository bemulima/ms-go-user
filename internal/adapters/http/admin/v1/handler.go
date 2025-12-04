package v1

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/example/user-service/internal/adapters/http/middleware"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
	res "github.com/example/user-service/pkg/http"
)

type Handler struct {
	service service.UserManageService
}

func NewHandler(s service.UserManageService) *Handler {
	return &Handler{service: s}
}

type createManageUserRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
}

type updateManageUserRequest struct {
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type changeRoleRequest struct {
	Role string `json:"role"`
}

type changeStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateUser)
	g.PATCH("/:id", h.UpdateUser)
	g.PATCH("/:id/status", h.ChangeStatus)
	g.PATCH("/:id/role", h.ChangeRole)
}

func (h *Handler) CreateUser(c echo.Context) error {
	req := new(createManageUserRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", middleware.RequestIDFromCtx(c), nil)
	}
	status := domain.UserStatus(strings.ToUpper(strings.TrimSpace(req.Status)))
	user, err := h.service.CreateUser(c.Request().Context(), service.CreateUserRequest{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
		Role:        req.Role,
		Status:      status,
	})
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "create_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusCreated, user)
}

func (h *Handler) UpdateUser(c echo.Context) error {
	req := new(updateManageUserRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", middleware.RequestIDFromCtx(c), nil)
	}
	userID := c.Param("id")
	user, err := h.service.UpdateUser(c.Request().Context(), userID, service.UpdateUserRequest{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		return res.ErrorJSON(c, status, "update_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, user)
}

func (h *Handler) ChangeStatus(c echo.Context) error {
	req := new(changeStatusRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", middleware.RequestIDFromCtx(c), nil)
	}
	userID := c.Param("id")
	status := domain.UserStatus(strings.ToUpper(strings.TrimSpace(req.Status)))
	user, err := h.service.ChangeStatus(c.Request().Context(), userID, status)
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			statusCode = http.StatusNotFound
		}
		return res.ErrorJSON(c, statusCode, "status_change_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, user)
}

func (h *Handler) ChangeRole(c echo.Context) error {
	req := new(changeRoleRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", middleware.RequestIDFromCtx(c), nil)
	}
	userID := c.Param("id")
	if err := h.service.ChangeRole(c.Request().Context(), userID, req.Role); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		return res.ErrorJSON(c, status, "role_change_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, map[string]string{"status": "role_updated"})
}
