package internalhttp

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/example/user-service/internal/adapters/http/middleware"
	service "github.com/example/user-service/internal/usecase"
	res "github.com/example/user-service/pkg/http"
)

const maxResolveBodyBytes = 64 << 10

// Handler exposes private service-to-service user contracts.
type Handler struct {
	activeUsers service.ActiveUserResolver
}

// NewHandler creates the internal HTTP handler.
func NewHandler(activeUsers service.ActiveUserResolver) *Handler {
	return &Handler{activeUsers: activeUsers}
}

// Register mounts health and token-protected service-to-service endpoints.
func Register(g *echo.Group, handler *Handler, internalToken string) {
	g.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	protected := g.Group("/v1", requireInternalToken(internalToken))
	protected.POST("/users/active/resolve", handler.ResolveActiveUsers)
}

// ResolveActiveUsers validates and resolves one bounded UUID batch.
func (h *Handler) ResolveActiveUsers(c echo.Context) error {
	if h == nil || h.activeUsers == nil {
		return res.ErrorJSON(c, http.StatusServiceUnavailable, "service_unavailable", "active user resolver is unavailable", middleware.RequestIDFromCtx(c), nil)
	}
	request := struct {
		UserIDs []string `json:"user_ids"`
	}{}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxResolveBodyBytes)
	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return invalidRequest(c)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invalidRequest(c)
	}
	result, err := h.activeUsers.ResolveActiveUsers(c.Request().Context(), request.UserIDs)
	if err != nil {
		if errors.Is(err, service.ErrInvalidActiveUserBatch) {
			return invalidRequest(c)
		}
		return res.ErrorJSON(c, http.StatusInternalServerError, "resolve_failed", "active users could not be resolved", middleware.RequestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, result)
}

func requireInternalToken(expected string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			provided := c.Request().Header.Get("X-Internal-Token")
			if !secureEqual(strings.TrimSpace(expected), provided) {
				return res.ErrorJSON(c, http.StatusUnauthorized, "internal_authentication_required", "valid internal credentials are required", middleware.RequestIDFromCtx(c), nil)
			}
			return next(c)
		}
	}
}

func secureEqual(expected, provided string) bool {
	if expected == "" || len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func invalidRequest(c echo.Context) error {
	return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "user_ids must contain 1 to 1000 unique UUIDs", middleware.RequestIDFromCtx(c), nil)
}
