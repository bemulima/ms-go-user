package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	nats "github.com/nats-io/nats.go"

	"github.com/example/user-service/config"
	repo "github.com/example/user-service/internal/adapters/postgres"
	rbacclient "github.com/example/user-service/internal/adapters/rbac"
	res "github.com/example/user-service/pkg/http"
	pkglog "github.com/example/user-service/pkg/log"
)

const roleCheckSubject = "rbac.checkRole"

type AuthMiddleware struct {
	cfg    *config.Config
	logger pkglog.Logger
	rbac   rbacclient.Client
	users  repo.UserRepository
	nats   *nats.Conn
}

func NewAuthMiddleware(cfg *config.Config, logger pkglog.Logger, rbac rbacclient.Client, users repo.UserRepository, natsConn *nats.Conn) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg, logger: logger, rbac: rbac, users: users, nats: natsConn}
}

func (a *AuthMiddleware) Handler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Request().Header.Get(echo.HeaderAuthorization)
		if header == "" {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "missing token", RequestIDFromCtx(c), nil)
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid token", RequestIDFromCtx(c), nil)
		}

		userID, role, email, err := a.verifyWithAuthService(c.Request().Context(), parts[1])
		if err != nil {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", err.Error(), RequestIDFromCtx(c), nil)
		}

		user, err := a.users.FindByID(c.Request().Context(), userID)
		if err != nil || user == nil {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "user not found", RequestIDFromCtx(c), nil)
		}
		c.Set("user", user)
		c.Set("email", email)

		if a.nats != nil {
			allowed, err := a.checkRole(c.Request().Context(), userID, role)
			if err != nil {
				return res.ErrorJSON(c, http.StatusForbidden, "forbidden", err.Error(), RequestIDFromCtx(c), nil)
			}
			if !allowed {
				return res.ErrorJSON(c, http.StatusForbidden, "forbidden", "role not allowed", RequestIDFromCtx(c), nil)
			}
		}

		c.Set("user_id", userID)
		c.Set("role", strings.ToUpper(role))
		if a.rbac != nil {
			if perms, err := a.rbac.GetPermissionsByUserID(c.Request().Context(), userID); err == nil {
				c.Set("permissions", perms)
			}
		}
		return next(c)
	}
}

func (a *AuthMiddleware) checkRole(ctx context.Context, userID, role string) (bool, error) {
	if a.nats == nil {
		return false, errors.New("rbac nats connection not configured")
	}
	payload := struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}{UserID: userID, Role: role}
	data, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	msg, err := a.nats.RequestWithContext(ctx, roleCheckSubject, data)
	if err != nil {
		return false, err
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return false, err
	}
	if resp.Error != "" {
		return false, errors.New(resp.Error)
	}
	return resp.OK, nil
}

type verifyResp struct {
	OK     bool                   `json:"ok"`
	UserID string                 `json:"user_id"`
	Email  string                 `json:"email"`
	Claims map[string]interface{} `json:"claims"`
	Error  string                 `json:"error"`
}

func (a *AuthMiddleware) verifyWithAuthService(ctx context.Context, token string) (string, string, string, error) {
	if a.nats == nil {
		return "", "", "", errors.New("auth service not reachable")
	}
	payload := map[string]string{"token": token}
	data, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	msg, err := a.nats.RequestWithContext(ctx, a.cfg.NATSAuthVerify, data)
	if err != nil {
		return "", "", "", err
	}
	var resp verifyResp
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return "", "", "", err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "invalid token"
		}
		return "", "", "", errors.New(resp.Error)
	}
	role := ""
	if resp.Claims != nil {
		if r, ok := resp.Claims["role"].(string); ok {
			role = r
		}
	}
	return resp.UserID, role, resp.Email, nil
}
