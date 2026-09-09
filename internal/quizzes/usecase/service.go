package usecase

import (
	"context"
	"encoding/json"
	"time"
	"errors"
	"github.com/google/uuid"

	"github.com/equipo-mooc/plataforma-mooc/internal/quizzes/domain"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) CreateQuiz(
	ctx context.Context,
	resourceID uuid.UUID,
	passingScore float64,
	maxAttempts *int,
	timeLimitSeconds *int,
	feedbackMode string,
) (*domain.Quiz, error) {

	if resourceID == uuid.Nil {
		return nil, domain.ErrInvalidQuiz
	}

	if passingScore < 0 || passingScore > 100 {
		return nil, domain.ErrInvalidQuiz
	}

	if maxAttempts != nil && *maxAttempts <= 0 {
		return nil, domain.ErrInvalidQuiz
	}

	if timeLimitSeconds != nil && *timeLimitSeconds <= 0 {
		return nil, domain.ErrInvalidQuiz
	}

	switch feedbackMode {
	case domain.FeedbackNone,
		domain.FeedbackAfterSubmit,
		domain.FeedbackAfterPass:
	default:
		return nil, domain.ErrInvalidQuiz
	}

	quiz := &domain.Quiz{
		ResourceID:       resourceID,
		PassingScore:     passingScore,
		MaxAttempts:      maxAttempts,
		TimeLimitSeconds: timeLimitSeconds,
		FeedbackMode:     feedbackMode,
	}

	if err := s.repo.CreateQuiz(ctx, quiz); err != nil {
		return nil, err
	}

	return quiz, nil
}

func (s *Service) CreateQuestion(
	ctx context.Context,
	quizID uuid.UUID,
	text string,
	position int,
	points float64,
) (*domain.Question, error) {

	if quizID == uuid.Nil || text == "" {
		return nil, domain.ErrInvalidQuiz
	}

	if position <= 0 || points <= 0 {
		return nil, domain.ErrInvalidQuiz
	}

	if _, err := s.repo.GetQuizByID(ctx, quizID); err != nil {
		return nil, err
	}

	question := &domain.Question{
		QuizID:   quizID,
		Text:     text,
		Position: position,
		Points:   points,
	}

	if err := s.repo.CreateQuestion(ctx, question); err != nil {
		return nil, err
	}

	return question, nil
}

func (s *Service) CreateOption(
	ctx context.Context,
	questionID uuid.UUID,
	text string,
	position int,
	isCorrect bool,
) (*domain.QuestionOption, error) {

	if questionID == uuid.Nil || text == "" || position <= 0 {
		return nil, domain.ErrInvalidQuiz
	}

	if _, err := s.repo.GetQuestionByID(ctx, questionID); err != nil {
		return nil, err
	}

	option := &domain.QuestionOption{
		QuestionID: questionID,
		Text:       text,
		Position:   position,
		IsCorrect:  isCorrect,
	}

	if err := s.repo.CreateOption(ctx, option); err != nil {
		return nil, err
	}

	return option, nil
}

func (s *Service) GetQuizForAuthoring(
	ctx context.Context,
	quizID uuid.UUID,
) (*domain.Quiz, []domain.Question, error) {

	quiz, err := s.repo.GetQuizByID(ctx, quizID)
	if err != nil {
		return nil, nil, err
	}

	questions, err := s.repo.GetQuestionsByQuizID(ctx, quizID)
	if err != nil {
		return nil, nil, err
	}

	return quiz, questions, nil
}

type snapshotOption struct {
	ID       uuid.UUID `json:"id"`
	Text     string    `json:"text"`
	Position int       `json:"position"`
}

type snapshotQuestion struct {
	ID       uuid.UUID        `json:"id"`
	StableID uuid.UUID        `json:"stable_id"`
	Text     string           `json:"text"`
	Position int              `json:"position"`
	Points   float64          `json:"points"`
	Options  []snapshotOption `json:"options"`
}

type quizSnapshot struct {
	QuizID    uuid.UUID          `json:"quiz_id"`
	Questions []snapshotQuestion `json:"questions"`
}

func (s *Service) StartAttempt(
	ctx context.Context,
	quizID uuid.UUID,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
) (*domain.Attempt, error) {

	quiz, err := s.repo.GetQuizByID(ctx, quizID)
	if err != nil {
		return nil, err
	}

	if quiz.MaxAttempts != nil {
		count, err := s.repo.CountAttemptsByStudent(
			ctx,
			quizID,
			studentID,
		)
		if err != nil {
			return nil, err
		}

		if count >= *quiz.MaxAttempts {
			return nil, domain.ErrMaxAttemptsReached
		}
	}

	questions, err := s.repo.GetQuestionsByQuizID(ctx, quizID)
	if err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, domain.ErrInvalidQuiz
	}
		snapshot := quizSnapshot{
		QuizID:    quiz.ID,
		Questions: make([]snapshotQuestion, 0, len(questions)),
	}

	for _, question := range questions {
		sq := snapshotQuestion{
			ID:       question.ID,
			StableID: question.StableID,
			Text:     question.Text,
			Position: question.Position,
			Points:   question.Points,
			Options:  make([]snapshotOption, 0, len(question.Options)),
		}

		for _, option := range question.Options {
			sq.Options = append(sq.Options, snapshotOption{
				ID:       option.ID,
				Text:     option.Text,
				Position: option.Position,
			})
		}

		snapshot.Questions = append(snapshot.Questions, sq)
	}

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	attempt := &domain.Attempt{
		QuizID:       quiz.ID,
		StudentID:    studentID,
		EnrollmentID: enrollmentID,
		Status:       domain.AttemptStatusInProgress,
		Snapshot:     snapshotJSON,
		StartedAt:    now,
	}

	if quiz.TimeLimitSeconds != nil {
		expiresAt := now.Add(
			time.Duration(*quiz.TimeLimitSeconds) * time.Second,
		)
		attempt.ExpiresAt = &expiresAt
	}

	if err := s.repo.CreateAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	return attempt, nil
}

func (s *Service) SaveAnswer(
	ctx context.Context,
	attemptID uuid.UUID,
	studentID uuid.UUID,
	questionID uuid.UUID,
	selectedOptionID *uuid.UUID,
) (*domain.AttemptAnswer, error) {

	attempt, err := s.repo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if attempt.StudentID != studentID {
		return nil, domain.ErrForbidden
	}

	if attempt.Status == domain.AttemptStatusSubmitted {
		return nil, domain.ErrAttemptAlreadySubmitted
	}

	if attempt.Status == domain.AttemptStatusExpired {
		return nil, domain.ErrAttemptExpired
	}

	if attempt.ExpiresAt != nil && time.Now().After(*attempt.ExpiresAt) {
		attempt.Status = domain.AttemptStatusExpired

		if err := s.repo.UpdateAttempt(ctx, attempt); err != nil {
			return nil, err
		}

		return nil, domain.ErrAttemptExpired
	}

	question, err := s.repo.GetQuestionByID(ctx, questionID)
	if err != nil {
		return nil, err
	}

	if question.QuizID != attempt.QuizID {
		return nil, domain.ErrInvalidAnswer
	}

	answer := &domain.AttemptAnswer{
		AttemptID:        attemptID,
		QuestionID:       questionID,
		SelectedOptionID: selectedOptionID,
	}

	if err := s.repo.SaveAnswer(ctx, answer); err != nil {
		return nil, err
	}

	return answer, nil
}

func (s *Service) SubmitAttempt(
	ctx context.Context,
	attemptID uuid.UUID,
	studentID uuid.UUID,
	idempotencyKey string,
) (*domain.Attempt, error) {

	if idempotencyKey == "" {
		return nil, domain.ErrInvalidQuiz
	}

	// Si esta key ya fue usada por el estudiante,
	// devolvemos el resultado existente.
	existing, err := s.repo.GetAttemptByIdempotencyKey(
		ctx,
		studentID,
		idempotencyKey,
	)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, domain.ErrAttemptNotFound) {
		return nil, err
	}

	attempt, err := s.repo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if attempt.StudentID != studentID {
		return nil, domain.ErrForbidden
	}

	if attempt.Status == domain.AttemptStatusSubmitted {
		return attempt, nil
	}

	if attempt.Status == domain.AttemptStatusExpired {
		return nil, domain.ErrAttemptExpired
	}

	if attempt.ExpiresAt != nil && time.Now().After(*attempt.ExpiresAt) {
		attempt.Status = domain.AttemptStatusExpired

		if err := s.repo.UpdateAttempt(ctx, attempt); err != nil {
			return nil, err
		}

		return nil, domain.ErrAttemptExpired
	}

	quiz, err := s.repo.GetQuizByID(ctx, attempt.QuizID)
	if err != nil {
		return nil, err
	}

	questions, err := s.repo.GetQuestionsByQuizID(ctx, attempt.QuizID)
	if err != nil {
		return nil, err
	}

	answers, err := s.repo.GetAnswersByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

		answerMap := make(map[uuid.UUID]uuid.UUID)

	for _, answer := range answers {
		if answer.SelectedOptionID != nil {
			answerMap[answer.QuestionID] = *answer.SelectedOptionID
		}
	}

		var totalPoints float64
	var earnedPoints float64

	for _, question := range questions {
		totalPoints += question.Points

		selectedOptionID, answered := answerMap[question.ID]
		if !answered {
			continue
		}

		for _, option := range question.Options {
			if option.ID == selectedOptionID && option.IsCorrect {
				earnedPoints += question.Points
				break
			}
		}
	}

		if totalPoints <= 0 {
		return nil, domain.ErrInvalidQuiz
	}

	percentage := (earnedPoints / totalPoints) * 100
	passed := percentage >= quiz.PassingScore

	now := time.Now()

	attempt.Score = &earnedPoints
	attempt.Percentage = &percentage
	attempt.Passed = &passed
	attempt.Status = domain.AttemptStatusSubmitted
	attempt.SubmittedAt = &now
	attempt.IdempotencyKey = &idempotencyKey

	if err := s.repo.UpdateAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	return attempt, nil
}