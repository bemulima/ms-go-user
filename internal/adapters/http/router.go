package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/example/user-service/config"
	adminv1 "github.com/example/user-service/internal/adapters/http/admin/v1"
	apiv1 "github.com/example/user-service/internal/adapters/http/api/v1"
	internalhttp "github.com/example/user-service/internal/adapters/http/internal"
	authmw "github.com/example/user-service/internal/adapters/http/middleware"
	service "github.com/example/user-service/internal/usecase"
)

type Router struct {
	cfg          *config.Config
	apiHandler   *apiv1.Handler
	adminHandler *adminv1.Handler
	activeUsers  service.ActiveUserResolver
	authMW       *authmw.AuthMiddleware
	rbacMW       *authmw.RBACMiddleware
}

// NewRouter creates the HTTP composition for public, administrative, and internal routes.
func NewRouter(cfg *config.Config, apiHandler *apiv1.Handler, adminHandler *adminv1.Handler, activeUsers service.ActiveUserResolver, authMW *authmw.AuthMiddleware, rbacMW *authmw.RBACMiddleware) *Router {
	return &Router{cfg: cfg, apiHandler: apiHandler, adminHandler: adminHandler, activeUsers: activeUsers, authMW: authMW, rbacMW: rbacMW}
}

func (r *Router) Setup(e *echo.Echo) {
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{r.cfg.CORSAllowOrigins},
		AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderXRequestedWith},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
	}))
	internalGroup := e.Group("/internal")
	internalhttp.Register(internalGroup, internalhttp.NewHandler(r.activeUsers), r.cfg.InternalAPIToken)

	apiGroup := e.Group("/api/v1/users", r.authMW.Handler)
	apiv1.RegisterRoutes(apiGroup, r.apiHandler)

	adminGroup := e.Group("/admin/v1/users", r.authMW.Handler, r.rbacMW.RequireAnyRole("admin", "moderator"))
	adminv1.RegisterRoutes(adminGroup, r.adminHandler)
}
