package http

import (
	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

func RegisterRoutes(e *echo.Echo, h *Handler) {

	authoring := e.Group(
		"/api/v1",
		authctx.RequireRole(
			authctx.RoleTeacher,
			authctx.RoleAdmin,
		),
	)

	authoring.POST(
		"/courses/:courseID/badge",
		h.CreateBadge,
	)

	admin := e.Group(
		"/api/v1",
		authctx.RequireRole(authctx.RoleAdmin),
	)

	admin.POST(
		"/badge-issuances/:issuanceID/revoke",
		h.RevokeIssuance,
	)

	// Público
	e.GET(
		"/api/v1/badges/verify/:verificationCode",
		h.VerifyBadge,
	)
}