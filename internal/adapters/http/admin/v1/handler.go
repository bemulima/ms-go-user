package v1

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/example/user-service/internal/adapters/filestorage"
	"github.com/example/user-service/internal/adapters/http/middleware"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
	res "github.com/example/user-service/pkg/http"
)

type Handler struct {
	service service.UserManageService
	storage filestorage.Client
}

func NewHandler(s service.UserManageService, storage filestorage.Client) *Handler {
	return &Handler{service: s, storage: storage}
}

type createManageUserRequest struct {
	Email        string  `json:"email"`
	Password     string  `json:"password"`
	DisplayName  *string `json:"display_name"`
	AvatarFileID *string `json:"avatar_file_id"`
	Role         string  `json:"role"`
	Status       string  `json:"status"`
}

type updateManageUserRequest struct {
	Email        *string `json:"email"`
	Password     *string `json:"password"`
	DisplayName  *string `json:"display_name"`
	AvatarFileID *string `json:"avatar_file_id"`
}

type changeRoleRequest struct {
	Role string `json:"role"`
}

type changeStatusRequest struct {
	Status string `json:"status"`
}

type userResponse struct {
	ID           string            `json:"id"`
	Email        string            `json:"email"`
	Status       domain.UserStatus `json:"status"`
	IsActive     bool              `json:"is_active"`
	DisplayName  *string           `json:"display_name,omitempty"`
	AvatarFileID *string           `json:"avatar_file_id,omitempty"`
	AvatarURL    *string           `json:"avatar_url,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

const (
	defaultPerPage = 50
	minPerPage     = 10
	maxPerPage     = 100
)

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.ListUsers)
	g.GET("/:id", h.GetUser)
	g.POST("", h.CreateUser)
	g.PATCH("/:id", h.UpdateUser)
	g.PATCH("/:id/status", h.ChangeStatus)
	g.PATCH("/:id/role", h.ChangeRole)
}

func (h *Handler) ListUsers(c echo.Context) error {
	page := 1
	if raw := strings.TrimSpace(c.QueryParam("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid page", middleware.RequestIDFromCtx(c), nil)
		}
		page = value
	}

	per := defaultPerPage
	if raw := strings.TrimSpace(c.QueryParam("per")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < minPerPage || value > maxPerPage {
			return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid per", middleware.RequestIDFromCtx(c), nil)
		}
		per = value
	}

	offset := (page - 1) * per
	users, totalCount, err := h.service.ListUsers(c.Request().Context(), offset, per)
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "list_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	items := make([]*userResponse, 0, len(users))
	for idx := range users {
		items = append(items, h.newUserResponse(&users[idx]))
	}
	return res.JSON(c, http.StatusOK, map[string]interface{}{
		"totalCount": totalCount,
		"users":      items,
	})
}

func (h *Handler) GetUser(c echo.Context) error {
	userID := c.Param("id")
	user, err := h.service.GetUser(c.Request().Context(), userID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		return res.ErrorJSON(c, status, "get_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, h.newUserResponse(user))
}

func (h *Handler) CreateUser(c echo.Context) error {
	req := new(createManageUserRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", middleware.RequestIDFromCtx(c), nil)
	}
	status := domain.UserStatus(strings.ToUpper(strings.TrimSpace(req.Status)))
	user, err := h.service.CreateUser(c.Request().Context(), service.CreateUserRequest{
		Email:        req.Email,
		Password:     req.Password,
		DisplayName:  req.DisplayName,
		AvatarFileID: req.AvatarFileID,
		Role:         req.Role,
		Status:       status,
	})
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "create_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusCreated, h.newUserResponse(user))
}

func (h *Handler) UpdateUser(c echo.Context) error {
	req := new(updateManageUserRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", middleware.RequestIDFromCtx(c), nil)
	}
	userID := c.Param("id")
	user, err := h.service.UpdateUser(c.Request().Context(), userID, service.UpdateUserRequest{
		Email:        req.Email,
		Password:     req.Password,
		DisplayName:  req.DisplayName,
		AvatarFileID: req.AvatarFileID,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		return res.ErrorJSON(c, status, "update_failed", err.Error(), middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, h.newUserResponse(user))
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
	return res.JSON(c, http.StatusOK, h.newUserResponse(user))
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
	return res.JSON(c, http.StatusOK, map[string]string{"id": userID, "role": strings.ToUpper(strings.TrimSpace(req.Role))})
}

func (h *Handler) newUserResponse(user *domain.User) *userResponse {
	if user == nil {
		return nil
	}

	profile := h.decorateProfile(user.Profile)
	return &userResponse{
		ID:           user.ID,
		Email:        maskEmail(user.Email),
		Status:       user.StatusOrDefault(),
		IsActive:     user.IsActive,
		DisplayName:  profileField(profile, func(value *domain.UserProfile) *string { return value.DisplayName }),
		AvatarFileID: profileField(profile, func(value *domain.UserProfile) *string { return value.AvatarFileID }),
		AvatarURL:    profileField(profile, func(value *domain.UserProfile) *string { return value.AvatarURL }),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func (h *Handler) decorateProfile(profile *domain.UserProfile) *domain.UserProfile {
	if profile == nil || h.storage == nil {
		return profile
	}
	profile.WithAvatarURL(h.storage.DownloadURL)
	return profile
}

func profileField(profile *domain.UserProfile, selector func(*domain.UserProfile) *string) *string {
	if profile == nil {
		return nil
	}
	return selector(profile)
}

func maskEmail(email string) string {
	normalized := strings.TrimSpace(email)
	parts := strings.Split(normalized, "@")
	if len(parts) != 2 {
		return normalized
	}

	local, domain := parts[0], parts[1]
	localRunes := []rune(local)
	if len(localRunes) == 0 {
		return normalized
	}
	maskedLocal := string(localRunes[0]) + "****"

	domainName := domain
	domainSuffix := ""
	if dot := strings.LastIndex(domain, "."); dot > 0 && dot < len(domain)-1 {
		domainName = domain[:dot]
		domainSuffix = domain[dot:]
	}

	domainRunes := []rune(domainName)
	if len(domainRunes) == 0 {
		return maskedLocal + "@***" + domainSuffix
	}

	return maskedLocal + "@***" + string(domainRunes[len(domainRunes)-1]) + domainSuffix
}
