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
)

type Router struct {
	cfg          *config.Config
	apiHandler   *apiv1.Handler
	adminHandler *adminv1.Handler
	authMW       *authmw.AuthMiddleware
	rbacMW       *authmw.RBACMiddleware
}

func NewRouter(cfg *config.Config, apiHandler *apiv1.Handler, adminHandler *adminv1.Handler, authMW *authmw.AuthMiddleware, rbacMW *authmw.RBACMiddleware) *Router {
	return &Router{cfg: cfg, apiHandler: apiHandler, adminHandler: adminHandler, authMW: authMW, rbacMW: rbacMW}
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
	internalhttp.Register(e)

	userGroup := e.Group("/users", r.authMW.Handler)
	apiv1.RegisterRoutes(userGroup, r.apiHandler)

	adminGroup := e.Group("/admin/users", r.authMW.Handler, r.rbacMW.RequireAnyRole("admin", "moderator"))
	adminv1.RegisterRoutes(adminGroup, r.adminHandler)
	adminGroup.GET("/:id", r.apiHandler.GetByID)
}
