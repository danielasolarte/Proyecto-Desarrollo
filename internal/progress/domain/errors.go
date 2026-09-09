package domain

import "errors"

var (
	ErrProgressNotFound = errors.New("progress not found")
	ErrInvalidHeartbeat = errors.New("invalid heartbeat")
	ErrInvalidProgress  = errors.New("invalid progress event")
	ErrForbidden        = errors.New("forbidden")
	ErrEnrollmentNeeded = errors.New("active enrollment required")
)