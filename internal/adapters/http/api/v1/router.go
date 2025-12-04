package v1

import (
	"github.com/labstack/echo/v4"
)

// RegisterRoutes attaches user-facing endpoints under provided group.
func RegisterRoutes(g *echo.Group, h *Handler) {
	h.RegisterRoutes(g)
}
