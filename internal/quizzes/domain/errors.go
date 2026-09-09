package domain

import "errors"

var (
	ErrQuizNotFound          = errors.New("quiz not found")
	ErrQuestionNotFound      = errors.New("question not found")
	ErrAttemptNotFound       = errors.New("attempt not found")
	ErrAttemptExpired        = errors.New("attempt expired")
	ErrAttemptAlreadySubmitted = errors.New("attempt already submitted")
	ErrMaxAttemptsReached    = errors.New("maximum number of attempts reached")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden")
	ErrInvalidAnswer         = errors.New("invalid answer")
	ErrInvalidQuiz           = errors.New("invalid quiz")
	ErrOptionNotFound = errors.New("option not found")
	ErrIdempotencyConflict = errors.New("idempotency key already used for another attempt")
)