package profile

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/courtknights/courtknights/internal/api/common"
	"github.com/courtknights/courtknights/internal/domain/profile"
)

// Members accepted by PATCH /users/me/profile and inside "preferences".
// Anything else is rejected (ADR-015: unknown members → 400).
var (
	profilePatchMembers     = []string{"display_name", "city", "region", "country", "gender", "date_of_birth", "category", "preferences"}
	preferencesPatchMembers = []string{"court_side", "handedness"}
)

// parseProfilePatch converts the members of a JSON Merge Patch into a
// ProfilePatch (FEATURE_clearable_profile_fields, ADR-015): an omitted member
// leaves the field unchanged, null clears it and a value sets it.
// Every error describes the offending member and maps to 400.
func parseProfilePatch(members map[string]json.RawMessage) (profile.ProfilePatch, error) {
	if err := rejectUnknown(members, profilePatchMembers, ""); err != nil {
		return profile.ProfilePatch{}, err
	}

	var patch profile.ProfilePatch
	var err error

	if raw, ok := members["display_name"]; ok {
		if common.IsJSONNull(raw) {
			return profile.ProfilePatch{}, fmt.Errorf("display_name cannot be cleared")
		}
		name, err := parseString("display_name", raw)
		if err != nil {
			return profile.ProfilePatch{}, err
		}
		patch.DisplayName = &name
	}
	if patch.City, err = stringField("city", members); err != nil {
		return profile.ProfilePatch{}, err
	}
	if patch.Region, err = stringField("region", members); err != nil {
		return profile.ProfilePatch{}, err
	}
	if patch.Country, err = stringField("country", members); err != nil {
		return profile.ProfilePatch{}, err
	}
	if patch.Gender, err = enumField("gender", members, profile.GenderMale, profile.GenderFemale); err != nil {
		return profile.ProfilePatch{}, err
	}
	if patch.DateOfBirth, err = dateField("date_of_birth", members); err != nil {
		return profile.ProfilePatch{}, err
	}
	if patch.Category, err = enumField("category", members,
		profile.CategoryFirst, profile.CategorySecond, profile.CategoryThird,
		profile.CategoryFourth, profile.CategoryFifth); err != nil {
		return profile.ProfilePatch{}, err
	}
	if patch.Preferences, err = preferencesField(members); err != nil {
		return profile.ProfilePatch{}, err
	}
	return patch, nil
}

// preferencesField parses the "preferences" member: null clears every
// preference, an object is merged member by member.
func preferencesField(members map[string]json.RawMessage) (profile.PreferencesPatch, error) {
	raw, ok := members["preferences"]
	if !ok {
		return profile.PreferencesPatch{}, nil
	}
	if common.IsJSONNull(raw) {
		return profile.PreferencesPatch{Clear: true}, nil
	}
	var prefs map[string]json.RawMessage
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") || json.Unmarshal(raw, &prefs) != nil {
		return profile.PreferencesPatch{}, fmt.Errorf("preferences must be an object or null")
	}
	if err := rejectUnknown(prefs, preferencesPatchMembers, "preferences."); err != nil {
		return profile.PreferencesPatch{}, err
	}

	var patch profile.PreferencesPatch
	var err error
	if patch.CourtSide, err = enumField("preferences.court_side", renamed(prefs, "court_side", "preferences.court_side"),
		profile.CourtSideDrive, profile.CourtSideBackhand, profile.CourtSideBoth); err != nil {
		return profile.PreferencesPatch{}, err
	}
	if patch.Handedness, err = enumField("preferences.handedness", renamed(prefs, "handedness", "preferences.handedness"),
		profile.HandednessRight, profile.HandednessLeft); err != nil {
		return profile.PreferencesPatch{}, err
	}
	return patch, nil
}

// renamed returns a one-member map holding members[from] under the key to, so
// nested members are reported with their full path in error messages.
func renamed(members map[string]json.RawMessage, from, to string) map[string]json.RawMessage {
	raw, ok := members[from]
	if !ok {
		return nil
	}
	return map[string]json.RawMessage{to: raw}
}

// rejectUnknown returns an error naming the first unknown member (sorted, so
// the message is deterministic).
func rejectUnknown(members map[string]json.RawMessage, allowed []string, prefix string) error {
	var unknown []string
	for name := range members {
		if !slices.Contains(allowed, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("unknown member %q", prefix+unknown[0])
}

// stringField parses an optional, clearable string member.
func stringField(name string, members map[string]json.RawMessage) (profile.Field[string], error) {
	raw, ok := members[name]
	if !ok {
		return profile.Unchanged[string](), nil
	}
	if common.IsJSONNull(raw) {
		return profile.Cleared[string](), nil
	}
	s, err := parseString(name, raw)
	if err != nil {
		return profile.Field[string]{}, err
	}
	return profile.SetTo(s), nil
}

// enumField parses an optional, clearable string member restricted to the
// given values.
func enumField[T ~string](name string, members map[string]json.RawMessage, allowed ...T) (profile.Field[T], error) {
	f, err := stringField(name, members)
	if err != nil || f.Value == nil {
		return profile.Field[T]{Set: f.Set}, err
	}
	v := T(*f.Value)
	if !slices.Contains(allowed, v) {
		return profile.Field[T]{}, fmt.Errorf("invalid %s value: %q", name, *f.Value)
	}
	return profile.SetTo(v), nil
}

// dateField parses an optional, clearable YYYY-MM-DD member.
func dateField(name string, members map[string]json.RawMessage) (profile.Field[time.Time], error) {
	f, err := stringField(name, members)
	if err != nil || f.Value == nil {
		return profile.Field[time.Time]{Set: f.Set}, err
	}
	t, err := time.Parse(time.DateOnly, *f.Value)
	if err != nil {
		return profile.Field[time.Time]{}, fmt.Errorf("invalid %s: must be YYYY-MM-DD", name)
	}
	return profile.SetTo(t), nil
}

// parseString decodes a non-blank JSON string; null is handled by the caller.
func parseString(name string, raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("%s must be a string or null", name)
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%s must not be blank; use null to clear it", name)
	}
	return s, nil
}
