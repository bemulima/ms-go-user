package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/example/user-service/config"
	"github.com/example/user-service/internal/adapter/http/handlers"
	authmw "github.com/example/user-service/internal/adapter/http/middleware"
	res "github.com/example/user-service/pkg/http"
)

type Router struct {
	cfg           *config.Config
	authHandler   *handlers.AuthHandler
	userHandler   *handlers.UserHandler
	manageHandler *handlers.UserManageHandler
	authMW        *authmw.AuthMiddleware
	rbacMW        *authmw.RBACMiddleware
}

func NewRouter(cfg *config.Config, authHandler *handlers.AuthHandler, userHandler *handlers.UserHandler, manageHandler *handlers.UserManageHandler, authMW *authmw.AuthMiddleware, rbacMW *authmw.RBACMiddleware) *Router {
	return &Router{cfg: cfg, authHandler: authHandler, userHandler: userHandler, manageHandler: manageHandler, authMW: authMW, rbacMW: rbacMW}
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
	e.GET("/health", func(c echo.Context) error {
		return res.JSON(c, http.StatusOK, map[string]string{"status": "ok"})
	})

	authGroup := e.Group("/auth")
	r.authHandler.RegisterRoutes(authGroup)

	userGroup := e.Group("/users", r.authMW.Handler)
	r.userHandler.RegisterRoutes(userGroup)

	adminGroup := e.Group("/admin/users", r.authMW.Handler, r.rbacMW.RequireAnyRole("admin", "moderator"))
	r.manageHandler.RegisterRoutes(adminGroup)
	adminGroup.GET("/:id", r.userHandler.GetByID)
}
