package ckerrors

import "errors"

// Profile sentinel errors.
var (
	// ErrProfileNotFound is returned when a profile lookup produces no result.
	ErrProfileNotFound = errors.New("profile not found")

	// ErrInvalidLocation is returned when a country or region code fails ISO 3166 validation,
	// or when a region is provided without a country.
	ErrInvalidLocation = errors.New("invalid location")
)
