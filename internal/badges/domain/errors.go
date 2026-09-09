package domain

import "errors"

var (
	ErrBadgeNotFound         = errors.New("badge not found")
	ErrIssuanceNotFound      = errors.New("badge issuance not found")
	ErrBadgeAlreadyIssued    = errors.New("badge already issued")
	ErrBadgeInactive         = errors.New("badge inactive")
	ErrCourseNotApproved     = errors.New("course not approved")
	ErrForbidden             = errors.New("forbidden")
)