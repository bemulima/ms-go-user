package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	nats "github.com/nats-io/nats.go"

	"github.com/example/user-service/config"
	repo "github.com/example/user-service/internal/adapter/postgres"
	rbacclient "github.com/example/user-service/internal/adapter/rbac"
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
	hmac   []byte
	key    interface{}
}

func NewAuthMiddleware(cfg *config.Config, logger pkglog.Logger, rbac rbacclient.Client, users repo.UserRepository, natsConn *nats.Conn) *AuthMiddleware {
	mw := &AuthMiddleware{cfg: cfg, logger: logger, rbac: rbac, users: users, nats: natsConn}
	if cfg.JWTSecret != "" {
		mw.hmac = []byte(cfg.JWTSecret)
	}
	if cfg.JWTPublicKey != "" {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.JWTPublicKey))
		if err == nil {
			mw.key = key
		}
	}
	return mw
}

func (a *AuthMiddleware) Handler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Request().Header.Get(echo.HeaderAuthorization)
		if header == "" {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "missing token", requestIDFromCtx(c), nil)
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid token", requestIDFromCtx(c), nil)
		}

		claims := jwt.MapClaims{}
		parser := jwt.NewParser(
			jwt.WithAudience(a.cfg.JWTAudience),
			jwt.WithIssuer(a.cfg.JWTIssuer),
			jwt.WithLeeway(30*time.Second),
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg(), jwt.SigningMethodRS256.Alg()}),
		)
		token, err := parser.ParseWithClaims(parts[1], claims, a.keyFunc)
		if err != nil || token == nil || !token.Valid {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid token", requestIDFromCtx(c), nil)
		}
		if exp, err := claims.GetExpirationTime(); err != nil || exp == nil || time.Now().After(exp.Time) {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "token expired or invalid", requestIDFromCtx(c), nil)
		}

		subject, _ := claims["sub"].(string)
		if subject == "" {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid subject", requestIDFromCtx(c), nil)
		}
		role, _ := claims["role"].(string)
		if strings.TrimSpace(role) == "" {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "role missing in token", requestIDFromCtx(c), nil)
		}

		user, err := a.users.FindByID(c.Request().Context(), subject)
		if err != nil || user == nil {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "user not found", requestIDFromCtx(c), nil)
		}
		c.Set("user", user)

		if a.nats != nil {
			allowed, err := a.checkRole(c.Request().Context(), subject, role)
			if err != nil {
				return res.ErrorJSON(c, http.StatusForbidden, "forbidden", err.Error(), requestIDFromCtx(c), nil)
			}
			if !allowed {
				return res.ErrorJSON(c, http.StatusForbidden, "forbidden", "role not allowed", requestIDFromCtx(c), nil)
			}
		}

		c.Set("user_id", subject)
		c.Set("role", strings.ToUpper(role))
		if a.rbac != nil {
			if perms, err := a.rbac.GetPermissionsByUserID(c.Request().Context(), subject); err == nil {
				c.Set("permissions", perms)
			}
		}
		return next(c)
	}
}

func (a *AuthMiddleware) keyFunc(token *jwt.Token) (interface{}, error) {
	if a.hmac != nil {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return a.hmac, nil
	}
	if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
		return nil, errors.New("unexpected signing method")
	}
	return a.key, nil
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
