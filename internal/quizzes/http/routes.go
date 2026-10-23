package http

import (
	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group("/api/v1")

	// Autoría.
	authoring := g.Group(
		"",
		authctx.RequireRole(
			authctx.RoleTeacher,
			authctx.RoleAdmin,
		),
	)

	authoring.POST(
		"/resources/:resourceID/quiz",
		h.CreateQuiz,
	)

	authoring.POST(
		"/quizzes/:quizID/questions",
		h.CreateQuestion,
	)

	authoring.POST(
		"/questions/:questionID/options",
		h.CreateOption,
	)

	authoring.GET(
		"/quizzes/:quizID/authoring",
		h.GetQuizForAuthoring,
	)

	// Profesor/admin: quiz completo para el editor (404 si aún no existe).
	// Estudiante inscrito: id del quiz publicado para iniciar un intento.
	g.GET(
		"/resources/:resourceID/quiz",
		h.GetQuizByResource,
		authctx.RequireRole(authctx.RoleTeacher, authctx.RoleAdmin, authctx.RoleStudent),
	)

	// Estudiante.
	student := g.Group(
		"",
		authctx.RequireRole(authctx.RoleStudent),
	)

	student.POST(
		"/quizzes/:quizID/attempts",
		h.StartAttempt,
	)

	student.GET(
		"/attempts/:attemptID",
		h.GetAttempt,
	)

	student.PUT(
		"/attempts/:attemptID/answers/:questionID",
		h.SaveAnswer,
	)

	student.POST(
		"/attempts/:attemptID/submit",
		h.SubmitAttempt,
	)
}
