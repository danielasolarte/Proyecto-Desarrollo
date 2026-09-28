package http

import (
	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

// RegisterRoutes registra el catálogo público y las inscripciones. El
// catálogo se puede leer sin estar autenticado; inscribirse, retirarse y
// ver "mis inscripciones" exige ser estudiante autenticado.
func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group("/api/v1")

	g.GET("/catalog", h.GetCatalog)

	student := g.Group("", authctx.RequireRole(authctx.RoleStudent, authctx.RoleAdmin))
	student.POST("/courses/:courseID/enrollments", h.Enroll)
	student.DELETE("/courses/:courseID/enrollments", h.Withdraw)
	student.GET("/me/enrollments", h.ListMyEnrollments)
}
