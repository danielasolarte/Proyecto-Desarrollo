package domain

import (
	"time"

	"github.com/google/uuid"
)

type Badge struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	Name        string
	Description *string
	ImageURL    *string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BadgeIssuance struct {
	ID               uuid.UUID
	BadgeID          uuid.UUID
	StudentID        uuid.UUID
	CourseID         uuid.UUID
	EnrollmentID     uuid.UUID
	VerificationCode uuid.UUID
	IssuedAt         time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}