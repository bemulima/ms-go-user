package v1

import (
	"github.com/labstack/echo/v4"
)

// RegisterRoutes attaches admin user management endpoints.
func RegisterRoutes(g *echo.Group, h *Handler) {
	h.RegisterRoutes(g)
}
