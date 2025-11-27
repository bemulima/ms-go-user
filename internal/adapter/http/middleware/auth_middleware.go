package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"github.com/example/user-service/config"
	rbacclient "github.com/example/user-service/internal/adapter/rbac"
	res "github.com/example/user-service/pkg/http"
	pkglog "github.com/example/user-service/pkg/log"
)

type AuthMiddleware struct {
	cfg    *config.Config
	logger pkglog.Logger
	rbac   rbacclient.Client
	hmac   []byte
	key    interface{}
}

func NewAuthMiddleware(cfg *config.Config, logger pkglog.Logger, rbac rbacclient.Client) *AuthMiddleware {
	mw := &AuthMiddleware{cfg: cfg, logger: logger, rbac: rbac}
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
		token, err := jwt.Parse(parts[1], a.keyFunc)
		if err != nil || !token.Valid {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid token", requestIDFromCtx(c), nil)
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid token", requestIDFromCtx(c), nil)
		}
		subject, _ := claims["sub"].(string)
		if subject == "" {
			return res.ErrorJSON(c, http.StatusUnauthorized, "unauthorized", "invalid subject", requestIDFromCtx(c), nil)
		}
		c.Set("user_id", subject)
		if a.rbac != nil {
			if role, err := a.rbac.GetRoleByUserID(c.Request().Context(), subject); err == nil {
				c.Set("role", role)
			}
			if perms, err := a.rbac.GetPermissionsByUserID(c.Request().Context(), subject); err == nil {
				c.Set("permissions", perms)
			}
		}
		return next(c)
	}
}

func (a *AuthMiddleware) keyFunc(token *jwt.Token) (interface{}, error) {
	if a.hmac != nil {
		return a.hmac, nil
	}
	return a.key, nil
}
