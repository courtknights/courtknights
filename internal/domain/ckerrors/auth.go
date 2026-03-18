// Package ckerrors defines sentinel errors shared across the domain layer.
// No external imports are allowed in this package.
package ckerrors

import "errors"

// Authentication and authorisation sentinel errors.
var (
	// ErrUserNotFound is returned when a user lookup produces no result.
	ErrUserNotFound = errors.New("user not found")

	// ErrPATNotFound is returned when a PAT lookup produces no result.
	ErrPATNotFound = errors.New("personal access token not found")

	// ErrPATExpired is returned when a PAT exists but has passed its expiry time.
	ErrPATExpired = errors.New("personal access token expired")

	// ErrInvalidPAT is returned when the provided raw token does not match any stored hash.
	ErrInvalidPAT = errors.New("invalid personal access token")

	// ErrUnauthorized is returned when a request lacks valid credentials.
	ErrUnauthorized = errors.New("unauthorized")
)
