package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/equipo-mooc/plataforma-mooc/internal/progress/domain"
	coursesDomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

const (
	maxHeartbeatGapSeconds = 60
	minCompletionSeconds   = 30
)

type Service struct {
	repo       domain.Repository
	courseRepo coursesDomain.CourseRepository
}

func NewService(
	repo domain.Repository,
	courseRepo coursesDomain.CourseRepository,
) *Service {
	return &Service{
		repo:       repo,
		courseRepo: courseRepo,
	}
}

func (s *Service) OpenResource(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceID uuid.UUID,
	resourceStableID uuid.UUID,
) (*domain.ResourceProgress, error) {

	now := time.Now()

	progress, err := s.repo.GetResourceProgress(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
	)

	if err != nil && !errors.Is(err, domain.ErrProgressNotFound) {
		return nil, err
	}

	if errors.Is(err, domain.ErrProgressNotFound) {
		progress = &domain.ResourceProgress{
			StudentID:        studentID,
			EnrollmentID:     enrollmentID,
			ResourceID:       resourceID,
			ResourceStableID: resourceStableID,
			Status:           domain.ResourceStatusInProgress,
			OpenedAt:         &now,
			LastActivityAt:   &now,
			ActiveSeconds:    0,
		}
	} else {
		progress.ResourceID = resourceID

		if progress.Status == domain.ResourceStatusNotStarted {
			progress.Status = domain.ResourceStatusInProgress
		}

		if progress.OpenedAt == nil {
			progress.OpenedAt = &now
		}

		progress.LastActivityAt = &now
	}

	if err := s.repo.UpsertResourceProgress(ctx, progress); err != nil {
		return nil, err
	}

	event := &domain.ProgressEvent{
		StudentID:        studentID,
		EnrollmentID:     enrollmentID,
		ResourceID:       resourceID,
		ResourceStableID: resourceStableID,
		EventType:        domain.EventOpened,
	}

	if err := s.repo.CreateProgressEvent(ctx, event); err != nil {
		return nil, err
	}

	return progress, nil
}

func (s *Service) Heartbeat(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceID uuid.UUID,
	resourceStableID uuid.UUID,
	positionSeconds *int,
	clientTimestamp *time.Time,
) (*domain.ResourceProgress, error) {

	progress, err := s.repo.GetResourceProgress(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
	)
	if err != nil {
		return nil, err
	}

	if progress.Status == domain.ResourceStatusCompleted {
		return progress, nil
	}

	now := time.Now()

	if clientTimestamp != nil {
		if clientTimestamp.After(now.Add(5 * time.Minute)) {
			return nil, domain.ErrInvalidHeartbeat
		}
	}

	if positionSeconds != nil && *positionSeconds < 0 {
		return nil, domain.ErrInvalidHeartbeat
	}

	addSeconds := 0

	if progress.LastActivityAt != nil {
		gap := int(now.Sub(*progress.LastActivityAt).Seconds())

		if gap > 0 && gap <= maxHeartbeatGapSeconds {
			addSeconds = gap
		}
	}

	progress.ActiveSeconds += addSeconds
	progress.LastActivityAt = &now

	if positionSeconds != nil {
		if progress.LastPositionSeconds == nil ||
			*positionSeconds >= *progress.LastPositionSeconds {

			progress.LastPositionSeconds = positionSeconds
		}
	}

	if progress.Status == domain.ResourceStatusNotStarted {
		progress.Status = domain.ResourceStatusInProgress
	}

	if err := s.repo.UpsertResourceProgress(ctx, progress); err != nil {
		return nil, err
	}

	event := &domain.ProgressEvent{
		StudentID:        studentID,
		EnrollmentID:     enrollmentID,
		ResourceID:       resourceID,
		ResourceStableID: resourceStableID,
		EventType:        domain.EventHeartbeat,
		PositionSeconds:  positionSeconds,
		ClientTimestamp:  clientTimestamp,
	}

	if err := s.repo.CreateProgressEvent(ctx, event); err != nil {
		return nil, err
	}

	return progress, nil
}

func (s *Service) UpdatePosition(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceID uuid.UUID,
	resourceStableID uuid.UUID,
	positionSeconds int,
) (*domain.ResourceProgress, error) {

	if positionSeconds < 0 {
		return nil, domain.ErrInvalidProgress
	}

	progress, err := s.repo.GetResourceProgress(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
	)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if progress.LastPositionSeconds == nil ||
		positionSeconds >= *progress.LastPositionSeconds {

		progress.LastPositionSeconds = &positionSeconds
	}

	progress.LastActivityAt = &now

	if progress.Status == domain.ResourceStatusNotStarted {
		progress.Status = domain.ResourceStatusInProgress
	}

	if err := s.repo.UpsertResourceProgress(ctx, progress); err != nil {
		return nil, err
	}

	event := &domain.ProgressEvent{
		StudentID:        studentID,
		EnrollmentID:     enrollmentID,
		ResourceID:       resourceID,
		ResourceStableID: resourceStableID,
		EventType:        domain.EventPosition,
		PositionSeconds:  &positionSeconds,
	}

	if err := s.repo.CreateProgressEvent(ctx, event); err != nil {
		return nil, err
	}

	return progress, nil
}

func (s *Service) CompleteResource(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceID uuid.UUID,
	resourceStableID uuid.UUID,
) (*domain.ResourceProgress, error) {

	progress, err := s.repo.GetResourceProgress(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
	)
	if err != nil {
		return nil, err
	}

	if progress.Status == domain.ResourceStatusCompleted {
		return progress, nil
	}

	if progress.ActiveSeconds < minCompletionSeconds {
		return nil, domain.ErrInvalidProgress
	}

	now := time.Now()

	progress.Status = domain.ResourceStatusCompleted
	progress.CompletedAt = &now
	progress.LastActivityAt = &now

	if err := s.repo.UpsertResourceProgress(ctx, progress); err != nil {
		return nil, err
	}

	event := &domain.ProgressEvent{
		StudentID:        studentID,
		EnrollmentID:     enrollmentID,
		ResourceID:       resourceID,
		ResourceStableID: resourceStableID,
		EventType:        domain.EventCompleted,
	}

	if err := s.repo.CreateProgressEvent(ctx, event); err != nil {
		return nil, err
	}

	return progress, nil
}

func (s *Service) GetResourceProgress(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceStableID uuid.UUID,
) (*domain.ResourceProgress, error) {

	return s.repo.GetResourceProgress(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
	)
}

func (s *Service) GetCourseProgress(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	courseID uuid.UUID,
) (*domain.CourseProgress, error) {

	progress, err := s.repo.GetCourseProgress(
		ctx,
		studentID,
		enrollmentID,
		courseID,
	)

	if errors.Is(err, domain.ErrProgressNotFound) {
		return &domain.CourseProgress{
			StudentID:          studentID,
			EnrollmentID:       enrollmentID,
			CourseID:           courseID,
			Status:             domain.CourseStatusInProgress,
			ProgressPercentage: 0,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return progress, nil
}

func (s *Service) RecalculateCourseProgress(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	courseID uuid.UUID,
) (*domain.CourseProgress, error) {

	version, err := s.courseRepo.FindPublishedVersion(courseID.String())
	if err != nil {
		return nil, err
	}

	tree, err := s.courseRepo.LoadTree(version.ID)
	if err != nil {
		return nil, err
	}

	requiredStableIDs := make(map[uuid.UUID]struct{})
	requiredQuizStableIDs := make(map[uuid.UUID]struct{})

	for _, module := range tree.Modules {
		for _, unit := range module.Units {
			for _, resource := range unit.Resources {

				if !resource.Visible || !resource.Required {
					continue
				}

				stableID, err := uuid.Parse(resource.StableID)
				if err != nil {
					return nil, err
				}

				requiredStableIDs[stableID] = struct{}{}
				if resource.Type == coursesDomain.ResourceQuiz {
					requiredQuizStableIDs[stableID] = struct{}{}
				}
			}
		}
	}

	completedIDs, err := s.repo.ListCompletedResourceStableIDs(
		ctx,
		studentID,
		enrollmentID,
	)
	if err != nil {
		return nil, err
	}

	completedSet := make(map[uuid.UUID]struct{})

	for _, stableID := range completedIDs {
		completedSet[stableID] = struct{}{}
	}

	passedQuizIDs, err := s.repo.ListPassedQuizStableIDs(
		ctx,
		studentID,
		enrollmentID,
	)
	if err != nil {
		return nil, err
	}

	passedQuizSet := make(map[uuid.UUID]struct{})

	for _, stableID := range passedQuizIDs {
		passedQuizSet[stableID] = struct{}{}
	}

	allRequiredQuizzesPassed := true

	for stableID := range requiredQuizStableIDs {
		if _, ok := passedQuizSet[stableID]; !ok {
			allRequiredQuizzesPassed = false
			break
		}
	}

	totalRequired := len(requiredStableIDs)
	completedRequired := 0

	for stableID := range requiredStableIDs {
		if _, ok := completedSet[stableID]; ok {
			completedRequired++
		}
	}

	var percentage float64

	if totalRequired > 0 {
		percentage =
			(float64(completedRequired) / float64(totalRequired)) * 100
	}

	progress, err := s.repo.GetCourseProgress(
		ctx,
		studentID,
		enrollmentID,
		courseID,
	)

	if err != nil && !errors.Is(err, domain.ErrProgressNotFound) {
		return nil, err
	}

	if errors.Is(err, domain.ErrProgressNotFound) {
		progress = &domain.CourseProgress{
			StudentID:          studentID,
			EnrollmentID:       enrollmentID,
			CourseID:           courseID,
			Status:             domain.CourseStatusInProgress,
			ProgressPercentage: 0,
		}
	}

	progress.ProgressPercentage = percentage

	if totalRequired > 0 &&
		completedRequired == totalRequired {

		now := time.Now()

		if progress.CompletedAt == nil {
			progress.CompletedAt = &now
		}

		if allRequiredQuizzesPassed {
			progress.Status = domain.CourseStatusApproved

			if progress.ApprovedAt == nil {
				progress.ApprovedAt = &now
			}
		} else {
			progress.Status = domain.CourseStatusCompleted
		}
	}

	if err := s.repo.UpsertCourseProgress(ctx, progress); err != nil {
		return nil, err
	}

	return progress, nil
}

func (s *Service) RecordQuizResult(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceID uuid.UUID,
	resourceStableID uuid.UUID,
	passed bool,
) (*domain.ResourceProgress, error) {

	now := time.Now()

	progress, err := s.repo.GetResourceProgress(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
	)

	if err != nil && !errors.Is(err, domain.ErrProgressNotFound) {
		return nil, err
	}

	if errors.Is(err, domain.ErrProgressNotFound) {
		progress = &domain.ResourceProgress{
			StudentID:        studentID,
			EnrollmentID:     enrollmentID,
			ResourceID:       resourceID,
			ResourceStableID: resourceStableID,
			Status:           domain.ResourceStatusCompleted,
			OpenedAt:         &now,
			LastActivityAt:   &now,
			CompletedAt:      &now,
		}
	} else {
		progress.ResourceID = resourceID
		progress.Status = domain.ResourceStatusCompleted
		progress.LastActivityAt = &now

		if progress.OpenedAt == nil {
			progress.OpenedAt = &now
		}

		if progress.CompletedAt == nil {
			progress.CompletedAt = &now
		}
	}

	if err := s.repo.UpsertResourceProgress(ctx, progress); err != nil {
		return nil, err
	}

	submittedExists, err := s.repo.HasProgressEvent(
		ctx,
		studentID,
		enrollmentID,
		resourceStableID,
		domain.EventQuizSubmitted,
	)
	if err != nil {
		return nil, err
	}

	if !submittedExists {
		submittedEvent := &domain.ProgressEvent{
			StudentID:        studentID,
			EnrollmentID:     enrollmentID,
			ResourceID:       resourceID,
			ResourceStableID: resourceStableID,
			EventType:        domain.EventQuizSubmitted,
		}

		if err := s.repo.CreateProgressEvent(ctx, submittedEvent); err != nil {
			return nil, err
		}
	}

	if passed {
		passedExists, err := s.repo.HasProgressEvent(
			ctx,
			studentID,
			enrollmentID,
			resourceStableID,
			domain.EventQuizPassed,
		)
		if err != nil {
			return nil, err
		}

		if !passedExists {
			passedEvent := &domain.ProgressEvent{
				StudentID:        studentID,
				EnrollmentID:     enrollmentID,
				ResourceID:       resourceID,
				ResourceStableID: resourceStableID,
				EventType:        domain.EventQuizPassed,
			}

			if err := s.repo.CreateProgressEvent(ctx, passedEvent); err != nil {
				return nil, err
			}
		}
	}

	return progress, nil
}