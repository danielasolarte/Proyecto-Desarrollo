package http

import (
	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

// RegisterRoutes registra todos los endpoints de autoría de cursos.
// Todos exigen rol profesor o admin, salvo la previsualización y la
// lectura de un curso, que son de solo lectura y no mutan nada (el
// control de acceso más fino, como si el estudiante está inscrito para
// ver contenido no publico, se aplica en el módulo de catálogo).
func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group("/api/v1")

	authoring := g.Group("", authctx.RequireRole(authctx.RoleTeacher, authctx.RoleAdmin))

	authoring.POST("/courses", h.CreateCourse)
	g.GET("/courses/:courseID", h.GetCourse)
	g.GET("/courses/:courseID/preview", h.PreviewCourse)
	authoring.POST("/courses/:courseID/draft", h.CreateUpdateDraft)
	authoring.POST("/courses/:courseID/publish", h.PublishCourse)

	authoring.PATCH("/versions/:versionID", h.UpdateVersion)

	authoring.POST("/versions/:versionID/modules", h.CreateModule)
	authoring.PATCH("/modules/:moduleID", h.UpdateModule)
	authoring.DELETE("/modules/:moduleID", h.DeleteModule)

	authoring.POST("/modules/:moduleID/units", h.CreateUnit)
	authoring.PATCH("/units/:unitID", h.UpdateUnit)
	authoring.DELETE("/units/:unitID", h.DeleteUnit)

	authoring.POST("/units/:unitID/resources", h.CreateResource)
	authoring.PATCH("/resources/:resourceID", h.UpdateResource)
	authoring.DELETE("/resources/:resourceID", h.DeleteResource)

	authoring.PUT("/resources/:resourceID/content/draft", h.SaveContentDraft)
	authoring.GET("/resources/:resourceID/content/draft", h.GetContentDraft)
	authoring.POST("/resources/:resourceID/content/publish", h.PublishContentDraft)
}
