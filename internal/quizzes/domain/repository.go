package domain

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateQuiz(ctx context.Context, quiz *Quiz) error
	GetQuizByID(ctx context.Context, id uuid.UUID) (*Quiz, error)
	GetQuizByResourceID(ctx context.Context, resourceID uuid.UUID) (*Quiz, error)
	UpdateQuiz(ctx context.Context, quiz *Quiz) error
	DeleteQuiz(ctx context.Context, id uuid.UUID) error
	CreateQuestion(ctx context.Context, question *Question) error
	GetQuestionByID(ctx context.Context, id uuid.UUID) (*Question, error)
	UpdateQuestion(ctx context.Context, question *Question) error
	DeleteQuestion(ctx context.Context, id uuid.UUID) error
	CreateOption(ctx context.Context, option *QuestionOption) error
	GetOptionByID(ctx context.Context, id uuid.UUID) (*QuestionOption, error)
	UpdateOption(ctx context.Context, option *QuestionOption) error
	DeleteOption(ctx context.Context, id uuid.UUID) error
	GetQuestionsByQuizID(ctx context.Context, quizID uuid.UUID) ([]Question, error)
	CreateAttempt(ctx context.Context, attempt *Attempt) error
	GetAttemptByID(ctx context.Context, id uuid.UUID) (*Attempt, error)
	CountAttemptsByStudent(ctx context.Context, quizID, studentID uuid.UUID) (int, error)
	UpdateAttempt(ctx context.Context, attempt *Attempt) error
	SubmitAttemptIfInProgress(ctx context.Context, attempt *Attempt) (bool, error)
	SaveAnswer(ctx context.Context, answer *AttemptAnswer) error
	GetAnswersByAttemptID(ctx context.Context, attemptID uuid.UUID) ([]AttemptAnswer, error)
	GetAttemptByIdempotencyKey(ctx context.Context, studentID uuid.UUID, key string) (*Attempt, error)
}
