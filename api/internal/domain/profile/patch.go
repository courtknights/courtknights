package profile

import "time"

// Field is one member of a JSON Merge Patch (RFC 7396, ADR-015):
//   - zero value (Set == false): the member was omitted → leave unchanged
//   - Set == true, Value == nil: the member was null    → clear
//   - Set == true, Value != nil: the member had a value → set
type Field[T any] struct {
	// Set reports whether the member was present in the patch.
	Set bool
	// Value is the new value, or nil to clear the field.
	Value *T
}

// Unchanged returns a Field that leaves the stored value as it is.
func Unchanged[T any]() Field[T] {
	return Field[T]{}
}

// Cleared returns a Field that clears the stored value.
func Cleared[T any]() Field[T] {
	return Field[T]{Set: true}
}

// SetTo returns a Field that sets the stored value to v.
func SetTo[T any](v T) Field[T] {
	return Field[T]{Set: true, Value: &v}
}

// IsClear reports whether the field clears the stored value.
func (f Field[T]) IsClear() bool {
	return f.Set && f.Value == nil
}

// ProfilePatch is a JSON Merge Patch of a Profile.
type ProfilePatch struct {
	// DisplayName is mandatory: it can be changed but never cleared, so it
	// stays a plain pointer (nil = unchanged).
	DisplayName *string
	// City is the free-text city.
	City Field[string]
	// Region is the ISO 3166-2 subdivision code.
	Region Field[string]
	// Country is the ISO 3166-1 alpha-2 country code.
	Country Field[string]
	// Gender is the self-declared gender.
	Gender Field[Gender]
	// DateOfBirth is the date of birth.
	DateOfBirth Field[time.Time]
	// Category is the self-declared federative category.
	Category Field[Category]
	// Preferences is merged into the stored preferences.
	Preferences PreferencesPatch
}

// IsEmpty reports whether the patch changes nothing.
func (p ProfilePatch) IsEmpty() bool {
	return p.DisplayName == nil &&
		!p.City.Set && !p.Region.Set && !p.Country.Set &&
		!p.Gender.Set && !p.DateOfBirth.Set && !p.Category.Set &&
		p.Preferences.IsEmpty()
}

// PreferencesPatch is merged recursively into the stored preferences.
type PreferencesPatch struct {
	// Clear is true for "preferences": null. Every stored preference is
	// removed before the members below are applied.
	Clear bool
	// CourtSide sets or removes the court_side preference.
	CourtSide Field[CourtSide]
	// Handedness sets or removes the handedness preference.
	Handedness Field[Handedness]
}

// IsEmpty reports whether the preferences patch changes nothing.
func (p PreferencesPatch) IsEmpty() bool {
	return !p.Clear && !p.CourtSide.Set && !p.Handedness.Set
}
