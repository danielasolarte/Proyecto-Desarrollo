package domain

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateBadge(
		ctx context.Context,
		badge *Badge,
	) error

	GetBadgeByID(
		ctx context.Context,
		id uuid.UUID,
	) (*Badge, error)

	GetBadgeByCourseID(
		ctx context.Context,
		courseID uuid.UUID,
	) (*Badge, error)

	CreateIssuance(
		ctx context.Context,
		issuance *BadgeIssuance,
	) error

	GetIssuanceByStudentAndCourse(
		ctx context.Context,
		studentID uuid.UUID,
		courseID uuid.UUID,
	) (*BadgeIssuance, error)

	GetIssuanceByVerificationCode(
		ctx context.Context,
		code uuid.UUID,
	) (*BadgeIssuance, error)

	RevokeIssuance(
		ctx context.Context,
		issuanceID uuid.UUID,
	) error
}