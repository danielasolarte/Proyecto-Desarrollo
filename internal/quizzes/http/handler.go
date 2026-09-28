package http

import (
	"net/http"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	catalogDomain "github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"
	coursesDomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
	quizDomain "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/domain"
	progressUC "github.com/equipo-mooc/plataforma-mooc/internal/progress/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/quizzes/usecase"
)

type Handler struct {
	service     *usecase.Service
	quizRepo    quizDomain.Repository
	courseRepo  coursesDomain.CourseRepository
	catalogRepo catalogDomain.CatalogRepository
	progressService *progressUC.Service
}

func NewHandler(
	service *usecase.Service,
	quizRepo quizDomain.Repository,
	courseRepo coursesDomain.CourseRepository,
	catalogRepo catalogDomain.CatalogRepository,
	progressService *progressUC.Service,
) *Handler {
	return &Handler{
		service:         service,
		quizRepo:        quizRepo,
		courseRepo:      courseRepo,
		catalogRepo:     catalogRepo,
		progressService: progressService,
	}
}

func currentUser(c echo.Context) (authctx.User, error) {
	user, ok := authctx.UserFromContext(c)
	if !ok {
		return authctx.User{}, quizDomain.ErrUnauthorized
	}

	return user, nil
}

func parseUserID(user authctx.User) (uuid.UUID, error) {
	id, err := uuid.Parse(user.ID)
	if err != nil {
		return uuid.Nil, quizDomain.ErrUnauthorized
	}

	return id, nil
}

func (h *Handler) courseForResource(
	resourceID string,
) (*coursesDomain.Course, *coursesDomain.Resource, error) {

	resource, err := h.courseRepo.FindResourceByID(resourceID)
	if err != nil {
		return nil, nil, err
	}

	unit, err := h.courseRepo.FindUnitByID(resource.UnitID)
	if err != nil {
		return nil, nil, err
	}

	module, err := h.courseRepo.FindModuleByID(unit.ModuleID)
	if err != nil {
		return nil, nil, err
	}

	version, err := h.courseRepo.FindVersionByID(module.CourseVersionID)
	if err != nil {
		return nil, nil, err
	}

	course, err := h.courseRepo.FindCourseByID(version.CourseID)
	if err != nil {
		return nil, nil, err
	}

	return course, resource, nil
}

func (h *Handler) ensureResourceOwner(
	user authctx.User,
	resourceID string,
) error {

	course, _, err := h.courseForResource(resourceID)
	if err != nil {
		return err
	}

	if user.Role == authctx.RoleAdmin {
		return nil
	}

	if course.TeacherID != user.ID {
		return quizDomain.ErrForbidden
	}

	return nil
}

type createQuizRequest struct {
	PassingScore     float64 `json:"passing_score"`
	MaxAttempts      *int    `json:"max_attempts"`
	TimeLimitSeconds *int    `json:"time_limit_seconds"`
	FeedbackMode     string  `json:"feedback_mode"`
}

func (h *Handler) CreateQuiz(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	resourceIDString := c.Param("resourceID")

	resourceID, err := uuid.Parse(resourceIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "resource_id invalido"),
		)
	}

	if err := h.ensureResourceOwner(user, resourceIDString); err != nil {
		return respondError(c, err)
	}

	_, resource, err := h.courseForResource(resourceIDString)
	if err != nil {
		return respondError(c, err)
	}

	if resource.Type != coursesDomain.ResourceQuiz {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(
				apierror.CodeValidation,
				"el recurso no es de tipo quiz",
			),
		)
	}

	var req createQuizRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "cuerpo invalido"),
		)
	}

	quiz, err := h.service.CreateQuiz(
		c.Request().Context(),
		resourceID,
		req.PassingScore,
		req.MaxAttempts,
		req.TimeLimitSeconds,
		req.FeedbackMode,
	)

	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusCreated, quiz)
}

type createQuestionRequest struct {
	Text     string  `json:"text"`
	Position int     `json:"position"`
	Points   float64 `json:"points"`
}

func (h *Handler) CreateQuestion(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	quizID, err := uuid.Parse(c.Param("quizID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "quiz_id invalido"),
		)
	}

	quiz, err := h.quizRepo.GetQuizByID(
		c.Request().Context(),
		quizID,
	)
	if err != nil {
		return respondError(c, err)
	}

	if err := h.ensureResourceOwner(
		user,
		quiz.ResourceID.String(),
	); err != nil {
		return respondError(c, err)
	}

	var req createQuestionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "cuerpo invalido"),
		)
	}

	question, err := h.service.CreateQuestion(
		c.Request().Context(),
		quizID,
		req.Text,
		req.Position,
		req.Points,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusCreated, question)
}

type createOptionRequest struct {
	Text      string `json:"text"`
	Position  int    `json:"position"`
	IsCorrect bool   `json:"is_correct"`
}

func (h *Handler) CreateOption(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	questionID, err := uuid.Parse(c.Param("questionID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "question_id invalido"),
		)
	}

	question, err := h.quizRepo.GetQuestionByID(
		c.Request().Context(),
		questionID,
	)
	if err != nil {
		return respondError(c, err)
	}

	quiz, err := h.quizRepo.GetQuizByID(
		c.Request().Context(),
		question.QuizID,
	)
	if err != nil {
		return respondError(c, err)
	}

	if err := h.ensureResourceOwner(
		user,
		quiz.ResourceID.String(),
	); err != nil {
		return respondError(c, err)
	}

	var req createOptionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "cuerpo invalido"),
		)
	}

	option, err := h.service.CreateOption(
		c.Request().Context(),
		questionID,
		req.Text,
		req.Position,
		req.IsCorrect,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusCreated, option)
}

func (h *Handler) GetQuizForAuthoring(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	quizID, err := uuid.Parse(c.Param("quizID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "quiz_id invalido"),
		)
	}

	quiz, err := h.quizRepo.GetQuizByID(
		c.Request().Context(),
		quizID,
	)
	if err != nil {
		return respondError(c, err)
	}

	if err := h.ensureResourceOwner(
		user,
		quiz.ResourceID.String(),
	); err != nil {
		return respondError(c, err)
	}

	quizData, questions, err := h.service.GetQuizForAuthoring(
		c.Request().Context(),
		quizID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"quiz":      quizData,
		"questions": questions,
	})
}

func (h *Handler) activeEnrollment(
	studentID string,
	courseID string,
) (*catalogDomain.Enrollment, error) {

	enrollment, err := h.catalogRepo.FindEnrollment(studentID, courseID)
	if err != nil {
		return nil, quizDomain.ErrForbidden
	}

	if enrollment.Status != catalogDomain.EnrollmentActive {
		return nil, quizDomain.ErrForbidden
	}

	return enrollment, nil
}

func (h *Handler) versionForResource(
	resourceID string,
) (*coursesDomain.CourseVersion, error) {

	resource, err := h.courseRepo.FindResourceByID(resourceID)
	if err != nil {
		return nil, err
	}

	unit, err := h.courseRepo.FindUnitByID(resource.UnitID)
	if err != nil {
		return nil, err
	}

	module, err := h.courseRepo.FindModuleByID(unit.ModuleID)
	if err != nil {
		return nil, err
	}

	return h.courseRepo.FindVersionByID(module.CourseVersionID)
}

type attemptResponse struct {
	ID           uuid.UUID       `json:"id"`
	QuizID       uuid.UUID       `json:"quiz_id"`
	Status       string          `json:"status"`
	Score        *float64        `json:"score,omitempty"`
	Percentage   *float64        `json:"percentage,omitempty"`
	Passed       *bool           `json:"passed,omitempty"`
	Snapshot     json.RawMessage `json:"snapshot"`
	StartedAt    time.Time       `json:"started_at"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	SubmittedAt  *time.Time      `json:"submitted_at,omitempty"`
}

func toAttemptResponse(a *quizDomain.Attempt) attemptResponse {
	return attemptResponse{
		ID:          a.ID,
		QuizID:      a.QuizID,
		Status:      a.Status,
		Score:       a.Score,
		Percentage:  a.Percentage,
		Passed:      a.Passed,
		Snapshot:    json.RawMessage(a.Snapshot),
		StartedAt:   a.StartedAt,
		ExpiresAt:   a.ExpiresAt,
		SubmittedAt: a.SubmittedAt,
	}
}

func (h *Handler) StartAttempt(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	studentID, err := parseUserID(user)
	if err != nil {
		return respondError(c, err)
	}

	quizID, err := uuid.Parse(c.Param("quizID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "quiz_id invalido"),
		)
	}

	quiz, err := h.quizRepo.GetQuizByID(
		c.Request().Context(),
		quizID,
	)
	if err != nil {
		return respondError(c, err)
	}

	course, _, err := h.courseForResource(quiz.ResourceID.String())
	if err != nil {
		return respondError(c, err)
	}

	version, err := h.versionForResource(quiz.ResourceID.String())
	if err != nil {
		return respondError(c, err)
	}

	// El estudiante solo puede usar contenido publicado.
	if course.PublishedVersionID == nil ||
		*course.PublishedVersionID != version.ID {

		return c.JSON(
			http.StatusForbidden,
			apierror.New(
				apierror.CodeForbidden,
				"el quiz no pertenece a la version publicada",
			),
		)
	}

	enrollment, err := h.activeEnrollment(
		user.ID,
		course.ID,
	)
	if err != nil {
		return respondError(c, err)
	}

	enrollmentID, err := uuid.Parse(enrollment.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(
				apierror.CodeInternal,
				"enrollment_id invalido",
			),
		)
	}

	attempt, err := h.service.StartAttempt(
		c.Request().Context(),
		quizID,
		studentID,
		enrollmentID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(
		http.StatusCreated,
		toAttemptResponse(attempt),
	)
}

func (h *Handler) GetAttempt(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	studentID, err := parseUserID(user)
	if err != nil {
		return respondError(c, err)
	}

	attemptID, err := uuid.Parse(c.Param("attemptID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "attempt_id invalido"),
		)
	}

	attempt, answers, err := h.service.GetAttemptForStudent(
		c.Request().Context(),
		attemptID,
		studentID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"attempt": toAttemptResponse(attempt),
		"answers": answers,
	})
}

type saveAnswerRequest struct {
	SelectedOptionID *string `json:"selected_option_id"`
}

func (h *Handler) SaveAnswer(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	studentID, err := parseUserID(user)
	if err != nil {
		return respondError(c, err)
	}

	attemptID, err := uuid.Parse(c.Param("attemptID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "attempt_id invalido"),
		)
	}

	questionID, err := uuid.Parse(c.Param("questionID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "question_id invalido"),
		)
	}

	var req saveAnswerRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "cuerpo invalido"),
		)
	}

	var selectedOptionID *uuid.UUID

	if req.SelectedOptionID != nil {
		parsed, err := uuid.Parse(*req.SelectedOptionID)
		if err != nil {
			return c.JSON(
				http.StatusBadRequest,
				apierror.New(
					apierror.CodeValidation,
					"selected_option_id invalido",
				),
			)
		}

		selectedOptionID = &parsed
	}

	answer, err := h.service.SaveAnswer(
		c.Request().Context(),
		attemptID,
		studentID,
		questionID,
		selectedOptionID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, answer)
}

func (h *Handler) SubmitAttempt(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	studentID, err := parseUserID(user)
	if err != nil {
		return respondError(c, err)
	}

	attemptID, err := uuid.Parse(c.Param("attemptID"))
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "attempt_id invalido"),
		)
	}

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")

	if idempotencyKey == "" {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(
				apierror.CodeValidation,
				"Idempotency-Key es obligatorio",
			),
		)
	}

	attempt, err := h.service.SubmitAttempt(
		c.Request().Context(),
		attemptID,
		studentID,
		idempotencyKey,
	)
	if err != nil {
		return respondError(c, err)
	}

	quiz, err := h.quizRepo.GetQuizByID(
		c.Request().Context(),
		attempt.QuizID,
	)
	if err != nil {
		return respondError(c, err)
	}

	course, resource, err := h.courseForResource(
		quiz.ResourceID.String(),
	)
	if err != nil {
		return respondError(c, err)
	}

	resourceStableID, err := uuid.Parse(resource.StableID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(
				apierror.CodeInternal,
				"resource stable_id invalido",
			),
		)
	}

	courseID, err := uuid.Parse(course.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(
				apierror.CodeInternal,
				"course_id invalido",
			),
		)
	}

	passed := false

	if attempt.Passed != nil {
		passed = *attempt.Passed
	}

	_, err = h.progressService.RecordQuizResult(
		c.Request().Context(),
		attempt.StudentID,
		attempt.EnrollmentID,
		quiz.ResourceID,
		resourceStableID,
		passed,
	)
	if err != nil {
		return respondError(c, err)
	}

	_, err = h.progressService.RecalculateCourseProgress(
		c.Request().Context(),
		attempt.StudentID,
		attempt.EnrollmentID,
		courseID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(
		http.StatusOK,
		toAttemptResponse(attempt),
	)
}



