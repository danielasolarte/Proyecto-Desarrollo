package http

import (
	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group(
		"/api/v1",
		authctx.RequireRole(authctx.RoleStudent),
	)

	g.POST(
		"/resources/:resourceID/progress/open",
		h.OpenResource,
	)

	g.POST(
		"/resources/:resourceID/progress/heartbeat",
		h.Heartbeat,
	)

	g.POST(
		"/resources/:resourceID/progress/complete",
		h.CompleteResource,
	)

	g.GET(
		"/courses/:courseID/progress",
		h.GetCourseProgress,
	)
}