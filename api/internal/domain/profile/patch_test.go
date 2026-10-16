package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestField_Helpers(t *testing.T) {
	unchanged := Unchanged[string]()
	assert.False(t, unchanged.Set)
	assert.Nil(t, unchanged.Value)
	assert.False(t, unchanged.IsClear())

	cleared := Cleared[string]()
	assert.True(t, cleared.Set)
	assert.Nil(t, cleared.Value)
	assert.True(t, cleared.IsClear())

	set := SetTo("Madrid")
	assert.True(t, set.Set)
	if assert.NotNil(t, set.Value) {
		assert.Equal(t, "Madrid", *set.Value)
	}
	assert.False(t, set.IsClear())
}

func TestProfilePatch_IsEmpty(t *testing.T) {
	name := "Alice"
	tests := []struct {
		name  string
		patch ProfilePatch
		want  bool
	}{
		{name: "zero patch", patch: ProfilePatch{}, want: true},
		{name: "display name", patch: ProfilePatch{DisplayName: &name}},
		{name: "city set", patch: ProfilePatch{City: SetTo("Madrid")}},
		{name: "region cleared", patch: ProfilePatch{Region: Cleared[string]()}},
		{name: "country set", patch: ProfilePatch{Country: SetTo("ES")}},
		{name: "gender cleared", patch: ProfilePatch{Gender: Cleared[Gender]()}},
		{name: "date of birth set", patch: ProfilePatch{DateOfBirth: SetTo(time.Date(1990, 5, 14, 0, 0, 0, 0, time.UTC))}},
		{name: "category cleared", patch: ProfilePatch{Category: Cleared[Category]()}},
		{name: "preferences cleared", patch: ProfilePatch{Preferences: PreferencesPatch{Clear: true}}},
		{name: "court side set", patch: ProfilePatch{Preferences: PreferencesPatch{CourtSide: SetTo(CourtSideDrive)}}},
		{name: "handedness removed", patch: ProfilePatch{Preferences: PreferencesPatch{Handedness: Cleared[Handedness]()}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.patch.IsEmpty())
		})
	}
}
