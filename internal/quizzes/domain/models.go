package domain

import (
	"time"

	"github.com/google/uuid"
)

type Quiz struct {
	ID               uuid.UUID
	ResourceID       uuid.UUID
	PassingScore     float64
	MaxAttempts      *int
	TimeLimitSeconds *int
	FeedbackMode     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Question struct {
	ID        uuid.UUID
	QuizID    uuid.UUID
	StableID  uuid.UUID
	Text      string
	Position  int
	Points    float64
	Options   []QuestionOption
	CreatedAt time.Time
	UpdatedAt time.Time
}

type QuestionOption struct {
	ID         uuid.UUID
	QuestionID uuid.UUID
	Text       string
	Position   int
	IsCorrect  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Attempt struct {
	ID             uuid.UUID
	QuizID         uuid.UUID
	StudentID      uuid.UUID
	EnrollmentID   uuid.UUID
	Status         string
	Score          *float64
	Percentage     *float64
	Passed         *bool
	Snapshot       []byte
	IdempotencyKey *string
	StartedAt      time.Time
	ExpiresAt      *time.Time
	SubmittedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AttemptAnswer struct {
	ID               uuid.UUID
	AttemptID         uuid.UUID
	QuestionID        uuid.UUID
	SelectedOptionID  *uuid.UUID
	SavedAt           time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

const (
	FeedbackNone        = "none"
	FeedbackAfterSubmit = "after_submit"
	FeedbackAfterPass   = "after_pass"
)

const (
	AttemptStatusInProgress = "in_progress"
	AttemptStatusSubmitted  = "submitted"
	AttemptStatusExpired    = "expired"
)