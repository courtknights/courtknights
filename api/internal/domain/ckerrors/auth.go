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

	// ErrEmailAlreadyRegistered is returned when a login would give an email to
	// a second account: users.email is unique and one account cannot be linked
	// to several providers (#132).
	ErrEmailAlreadyRegistered = errors.New("email already registered")
)

// EmailAlreadyRegisteredError is ErrEmailAlreadyRegistered with the provider of
// the account that owns the email, so the user can be told how to sign in.
// Provider is empty when it must not be disclosed.
// errors.Is(err, ErrEmailAlreadyRegistered) reports true for it.
type EmailAlreadyRegisteredError struct {
	// Provider is the existing account's provider ("google", "github"), or "".
	Provider string
}

// Error implements the error interface.
func (e *EmailAlreadyRegisteredError) Error() string {
	if e.Provider == "" {
		return ErrEmailAlreadyRegistered.Error()
	}
	return ErrEmailAlreadyRegistered.Error() + " with provider " + e.Provider
}

// Unwrap returns ErrEmailAlreadyRegistered.
func (e *EmailAlreadyRegisteredError) Unwrap() error {
	return ErrEmailAlreadyRegistered
}
