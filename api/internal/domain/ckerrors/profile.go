package ckerrors

import "errors"

// Profile sentinel errors.
var (
	// ErrProfileNotFound is returned when a profile lookup produces no result.
	ErrProfileNotFound = errors.New("profile not found")
)
