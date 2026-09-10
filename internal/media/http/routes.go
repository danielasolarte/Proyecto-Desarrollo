package http

import (
	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

// RegisterRoutes registra los endpoints de carga y reproducción de
// multimedia. Iniciar/confirmar carga exige rol profesor o admin (es
// autoría); consultar estado y obtener la URL de reproducción solo exige
// estar autenticado, dejando el filtro fino de inscripción al módulo de
// catálogo/progreso, igual que se documenta en internal/courses/http/routes.go
// para la previsualización.
func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group("/api/v1")

	authoring := g.Group("", authctx.RequireRole(authctx.RoleTeacher, authctx.RoleAdmin))

	authoring.POST("/resources/:resourceID/uploads/initiate", h.InitiateUpload)
	authoring.POST("/uploads/:uploadSessionID/complete", h.CompleteUpload)

	g.GET("/resources/:resourceID/media", h.GetMediaStatus)
	g.GET("/resources/:resourceID/playback", h.GetPlaybackURL)
}
