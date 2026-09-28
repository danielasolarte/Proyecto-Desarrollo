package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	badgeDomain "github.com/equipo-mooc/plataforma-mooc/internal/badges/domain"
	progressDomain "github.com/equipo-mooc/plataforma-mooc/internal/progress/domain"
)

type Service struct {
	repo         badgeDomain.Repository
	progressRepo progressDomain.Repository
}

func NewService(
	repo badgeDomain.Repository,
	progressRepo progressDomain.Repository,
) *Service {
	return &Service{
		repo:         repo,
		progressRepo: progressRepo,
	}
}

func (s *Service) CreateBadge(
	ctx context.Context,
	courseID uuid.UUID,
	name string,
	description *string,
	imageURL *string,
) (*badgeDomain.Badge, error) {

	if courseID == uuid.Nil || name == "" {
		return nil, badgeDomain.ErrBadgeNotFound
	}

	existing, err := s.repo.GetBadgeByCourseID(ctx, courseID)

	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, badgeDomain.ErrBadgeNotFound) {
		return nil, err
	}

	badge := &badgeDomain.Badge{
		CourseID:    courseID,
		Name:        name,
		Description: description,
		ImageURL:    imageURL,
		Active:      true,
	}

	if err := s.repo.CreateBadge(ctx, badge); err != nil {
		return nil, err
	}

	return badge, nil
}

func (s *Service) IssueBadgeIfApproved(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	courseID uuid.UUID,
) (*badgeDomain.BadgeIssuance, error) {

	courseProgress, err := s.progressRepo.GetCourseProgress(
		ctx,
		studentID,
		enrollmentID,
		courseID,
	)
	if err != nil {
		return nil, err
	}

	if courseProgress.Status != progressDomain.CourseStatusApproved {
		return nil, badgeDomain.ErrCourseNotApproved
	}

	existing, err := s.repo.GetIssuanceByStudentAndCourse(
		ctx,
		studentID,
		courseID,
	)

	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, badgeDomain.ErrIssuanceNotFound) {
		return nil, err
	}

	badge, err := s.repo.GetBadgeByCourseID(
		ctx,
		courseID,
	)
	if err != nil {
		return nil, err
	}

	if !badge.Active {
		return nil, badgeDomain.ErrBadgeInactive
	}

	issuance := &badgeDomain.BadgeIssuance{
		BadgeID:      badge.ID,
		StudentID:    studentID,
		CourseID:     courseID,
		EnrollmentID: enrollmentID,
	}

	if err := s.repo.CreateIssuance(ctx, issuance); err != nil {
		return nil, err
	}

	return issuance, nil
}

func (s *Service) EnsureBadgeIssued(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	courseID uuid.UUID,
) error {

	_, err := s.IssueBadgeIfApproved(
		ctx,
		studentID,
		enrollmentID,
		courseID,
	)

	if errors.Is(err, badgeDomain.ErrBadgeNotFound) {
		return nil
	}

	return err
}

type PublicVerification struct {
	Valid            bool      `json:"valid"`
	BadgeName        string    `json:"badge_name"`
	Description      *string   `json:"description,omitempty"`
	ImageURL         *string   `json:"image_url,omitempty"`
	CourseID         uuid.UUID `json:"course_id"`
	VerificationCode uuid.UUID `json:"verification_code"`
	IssuedAt         time.Time `json:"issued_at"`
	Revoked          bool      `json:"revoked"`
}

func (s *Service) Verify(
	ctx context.Context,
	code uuid.UUID,
) (*PublicVerification, error) {

	issuance, err := s.repo.GetIssuanceByVerificationCode(
		ctx,
		code,
	)
	if err != nil {
		return nil, err
	}

	badge, err := s.repo.GetBadgeByID(
		ctx,
		issuance.BadgeID,
	)
	if err != nil {
		return nil, err
	}

	revoked := issuance.RevokedAt != nil

	result := &PublicVerification{
		Valid:            !revoked && badge.Active,
		BadgeName:        badge.Name,
		Description:      badge.Description,
		ImageURL:         badge.ImageURL,
		CourseID:         issuance.CourseID,
		VerificationCode: issuance.VerificationCode,
		IssuedAt:         issuance.IssuedAt,
		Revoked:          revoked,
	}

	return result, nil
}

func (s *Service) Revoke(
	ctx context.Context,
	issuanceID uuid.UUID,
) error {

	if issuanceID == uuid.Nil {
		return badgeDomain.ErrIssuanceNotFound
	}

	return s.repo.RevokeIssuance(
		ctx,
		issuanceID,
	)
}