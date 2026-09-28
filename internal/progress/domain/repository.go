package domain

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetResourceProgress(
		ctx context.Context,
		studentID uuid.UUID,
		enrollmentID uuid.UUID,
		resourceStableID uuid.UUID,
	) (*ResourceProgress, error)

	UpsertResourceProgress(
		ctx context.Context,
		progress *ResourceProgress,
	) error

	CreateProgressEvent(
		ctx context.Context,
		event *ProgressEvent,
	) error

	ListCompletedResourceStableIDs(
		ctx context.Context,
		studentID uuid.UUID,
		enrollmentID uuid.UUID,
	) ([]uuid.UUID, error)

	GetCourseProgress(
		ctx context.Context,
		studentID uuid.UUID,
		enrollmentID uuid.UUID,
		courseID uuid.UUID,
	) (*CourseProgress, error)

	UpsertCourseProgress(
		ctx context.Context,
		progress *CourseProgress,
	) error

	ListPassedQuizStableIDs(
		ctx context.Context,
		studentID uuid.UUID,
		enrollmentID uuid.UUID,
	) ([]uuid.UUID, error)

	HasProgressEvent(
		ctx context.Context,
		studentID uuid.UUID,
		enrollmentID uuid.UUID,
		resourceStableID uuid.UUID,
		eventType string,
	) (bool, error)


}