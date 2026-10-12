package ckerrors

import "errors"

// Profile sentinel errors.
var (
	// ErrProfileNotFound is returned when a profile lookup produces no result.
	ErrProfileNotFound = errors.New("profile not found")

	// ErrInvalidLocation is returned when a country or region code fails ISO 3166 validation,
	// or when a region is provided without a country.
	ErrInvalidLocation = errors.New("invalid location")

	// ErrProfileConflict is returned when a profile write violates a consistency
	// constraint because another request changed the profile concurrently
	// (e.g. the region/country check). The client can retry.
	ErrProfileConflict = errors.New("profile changed concurrently")
)
