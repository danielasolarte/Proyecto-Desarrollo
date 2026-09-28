package domain

import (
	"time"

	"github.com/google/uuid"
)

type ResourceProgress struct {
	ID                  uuid.UUID
	StudentID           uuid.UUID
	EnrollmentID        uuid.UUID
	ResourceID          uuid.UUID
	ResourceStableID    uuid.UUID
	Status              string
	OpenedAt            *time.Time
	LastActivityAt      *time.Time
	CompletedAt         *time.Time
	ActiveSeconds       int
	LastPositionSeconds *int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ProgressEvent struct {
	ID               uuid.UUID
	StudentID        uuid.UUID
	EnrollmentID     uuid.UUID
	ResourceID       uuid.UUID
	ResourceStableID uuid.UUID
	EventType        string
	PositionSeconds  *int
	ClientTimestamp  *time.Time
	CreatedAt        time.Time
}

type CourseProgress struct {
	ID                 uuid.UUID
	StudentID          uuid.UUID
	EnrollmentID       uuid.UUID
	CourseID           uuid.UUID
	Status             string
	ProgressPercentage float64
	CompletedAt        *time.Time
	ApprovedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

const (
	ResourceStatusNotStarted = "not_started"
	ResourceStatusInProgress = "in_progress"
	ResourceStatusCompleted  = "completed"
)

const (
	CourseStatusInProgress = "in_progress"
	CourseStatusCompleted  = "completed"
	CourseStatusApproved   = "approved"
)

const (
	EventOpened        = "opened"
	EventHeartbeat     = "heartbeat"
	EventPosition      = "position"
	EventCompleted     = "completed"
	EventQuizSubmitted = "quiz_submitted"
	EventQuizPassed    = "quiz_passed"
)

