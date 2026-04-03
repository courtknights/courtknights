//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// createTestUser inserts a user into the test DB and returns its ID.
func createTestUser(t *testing.T, name string) uuid.UUID {
	t.Helper()
	repo := NewUserRepository(testDB)
	u, err := repo.Upsert(context.Background(), &user.User{
		Email:      fmt.Sprintf("%s@example.com", uuid.New().String()),
		Name:       name,
		Role:       user.RoleUser,
		Provider:   user.ProviderGoogle,
		ProviderID: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("createTestUser: Upsert: %v", err)
	}
	return u.ID
}

// ptr returns a pointer to the given value (generic helper).
func ptr[T any](v T) *T { return &v }

// ---- ProfileRepository: EnsureExists ----

// PR-01: Creates a profile for a new user.
func TestProfileRepository_EnsureExists_CreatesProfile(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Alice")
	repo := NewProfileRepository(testDB)

	err := repo.EnsureExists(context.Background(), userID, "Alice")
	if err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}

	p, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID after EnsureExists: %v", err)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("display_name: got %q, want %q", p.DisplayName, "Alice")
	}
	if p.UserID != userID {
		t.Errorf("user_id mismatch: got %v, want %v", p.UserID, userID)
	}
}

// PR-02: No-op when profile already exists.
func TestProfileRepository_EnsureExists_NoOpWhenAlreadyExists(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Bob")
	repo := NewProfileRepository(testDB)

	if err := repo.EnsureExists(context.Background(), userID, "Bob"); err != nil {
		t.Fatalf("first EnsureExists: %v", err)
	}

	// Update the display name directly so we can detect if EnsureExists overwrites it.
	_, err := testDB.Exec(context.Background(),
		`UPDATE user_profiles SET display_name = 'Bob Updated' WHERE user_id = $1`, userID)
	if err != nil {
		t.Fatalf("update display_name: %v", err)
	}

	// Second EnsureExists must be a no-op.
	if err := repo.EnsureExists(context.Background(), userID, "Bob"); err != nil {
		t.Fatalf("second EnsureExists: %v", err)
	}

	p, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if p.DisplayName != "Bob Updated" {
		t.Errorf("expected display_name unchanged at %q, got %q", "Bob Updated", p.DisplayName)
	}
}

// PR-03: Non-existent user ID causes FK violation error.
func TestProfileRepository_EnsureExists_FKViolationForUnknownUser(t *testing.T) {
	truncate(t)
	repo := NewProfileRepository(testDB)

	err := repo.EnsureExists(context.Background(), uuid.New(), "Ghost")
	if err == nil {
		t.Fatal("expected FK violation error, got nil")
	}
}

// ---- ProfileRepository: FindByUserID ----

// PR-04: Returns profile for existing user.
func TestProfileRepository_FindByUserID_ReturnsProfile(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Carol")
	repo := NewProfileRepository(testDB)

	if err := repo.EnsureExists(context.Background(), userID, "Carol"); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}

	p, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if p.UserID != userID {
		t.Errorf("user_id: got %v, want %v", p.UserID, userID)
	}
	if p.DisplayName != "Carol" {
		t.Errorf("display_name: got %q, want %q", p.DisplayName, "Carol")
	}
}

// PR-05: Returns ErrProfileNotFound when no profile exists.
func TestProfileRepository_FindByUserID_NotFound(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Dave")
	repo := NewProfileRepository(testDB)

	_, err := repo.FindByUserID(context.Background(), userID)
	if err == nil {
		t.Fatal("expected ErrProfileNotFound, got nil")
	}
	if !errors.Is(err, ckerrors.ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got: %v", err)
	}
}

// PR-06: All nullable fields are null when only display_name is set.
func TestProfileRepository_FindByUserID_NullableFieldsAreNil(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Eve")
	repo := NewProfileRepository(testDB)

	if err := repo.EnsureExists(context.Background(), userID, "Eve"); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}

	p, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if p.City != nil {
		t.Errorf("expected City nil, got %v", p.City)
	}
	if p.Region != nil {
		t.Errorf("expected Region nil, got %v", p.Region)
	}
	if p.Country != nil {
		t.Errorf("expected Country nil, got %v", p.Country)
	}
	if p.Gender != nil {
		t.Errorf("expected Gender nil, got %v", p.Gender)
	}
	if p.DateOfBirth != nil {
		t.Errorf("expected DateOfBirth nil, got %v", p.DateOfBirth)
	}
	if p.Category != nil {
		t.Errorf("expected Category nil, got %v", p.Category)
	}
}

// ---- ProfileRepository: Update ----

// PR-07: Partial update — only display_name; other fields unchanged.
func TestProfileRepository_Update_PartialUpdateDisplayName(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Frank")
	repo := NewProfileRepository(testDB)

	if err := repo.EnsureExists(context.Background(), userID, "Frank"); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}

	// Set city first so we can confirm it is unchanged after update.
	city := "Madrid"
	_, err := repo.Update(context.Background(), userID, profile.ProfilePatch{City: &city})
	if err != nil {
		t.Fatalf("pre-update city: %v", err)
	}

	updated, err := repo.Update(context.Background(), userID, profile.ProfilePatch{
		DisplayName: ptr("Frank Updated"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DisplayName != "Frank Updated" {
		t.Errorf("display_name: got %q, want %q", updated.DisplayName, "Frank Updated")
	}
	if updated.City == nil || *updated.City != "Madrid" {
		t.Errorf("city should remain %q, got %v", "Madrid", updated.City)
	}
}

// PR-08: Update all fields at once.
func TestProfileRepository_Update_AllFields(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Grace")
	repo := NewProfileRepository(testDB)

	if err := repo.EnsureExists(context.Background(), userID, "Grace"); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}

	dob := time.Date(1990, 5, 14, 0, 0, 0, 0, time.UTC)
	patch := profile.ProfilePatch{
		DisplayName: ptr("Grace Full"),
		City:        ptr("Barcelona"),
		Region:      ptr("ES-CT"),
		Country:     ptr("ES"),
		Gender:      ptr(profile.GenderFemale),
		DateOfBirth: &dob,
		Category:    ptr(profile.CategoryThird),
		Preferences: &profile.Preferences{
			CourtSide:  ptr(profile.CourtSideDrive),
			Handedness: ptr(profile.HandednessRight),
		},
	}

	updated, err := repo.Update(context.Background(), userID, patch)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.DisplayName != "Grace Full" {
		t.Errorf("display_name: got %q", updated.DisplayName)
	}
	if updated.City == nil || *updated.City != "Barcelona" {
		t.Errorf("city: got %v", updated.City)
	}
	if updated.Region == nil || *updated.Region != "ES-CT" {
		t.Errorf("region: got %v", updated.Region)
	}
	if updated.Country == nil || *updated.Country != "ES" {
		t.Errorf("country: got %v", updated.Country)
	}
	if updated.Gender == nil || *updated.Gender != profile.GenderFemale {
		t.Errorf("gender: got %v", updated.Gender)
	}
	if updated.DateOfBirth == nil {
		t.Fatal("expected DateOfBirth set")
	}
	// Compare date only (DB stores DATE, not full timestamp).
	gotDOB := updated.DateOfBirth.UTC()
	if gotDOB.Year() != 1990 || gotDOB.Month() != 5 || gotDOB.Day() != 14 {
		t.Errorf("date_of_birth: got %v, want 1990-05-14", gotDOB)
	}
	if updated.Category == nil || *updated.Category != profile.CategoryThird {
		t.Errorf("category: got %v", updated.Category)
	}
	if updated.Preferences.CourtSide == nil || *updated.Preferences.CourtSide != profile.CourtSideDrive {
		t.Errorf("preferences.court_side: got %v", updated.Preferences.CourtSide)
	}
	if updated.Preferences.Handedness == nil || *updated.Preferences.Handedness != profile.HandednessRight {
		t.Errorf("preferences.handedness: got %v", updated.Preferences.Handedness)
	}
}

// PR-09: Update preferences only; other fields unchanged.
func TestProfileRepository_Update_PreferencesOnly(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Henry")
	repo := NewProfileRepository(testDB)

	if err := repo.EnsureExists(context.Background(), userID, "Henry"); err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}

	// First set a city so we can confirm it remains unchanged.
	_, err := repo.Update(context.Background(), userID, profile.ProfilePatch{City: ptr("Seville")})
	if err != nil {
		t.Fatalf("pre-update city: %v", err)
	}

	updated, err := repo.Update(context.Background(), userID, profile.ProfilePatch{
		Preferences: &profile.Preferences{CourtSide: ptr(profile.CourtSideDrive)},
	})
	if err != nil {
		t.Fatalf("Update preferences: %v", err)
	}
	if updated.Preferences.CourtSide == nil || *updated.Preferences.CourtSide != profile.CourtSideDrive {
		t.Errorf("preferences.court_side: got %v", updated.Preferences.CourtSide)
	}
	if updated.City == nil || *updated.City != "Seville" {
		t.Errorf("city should remain %q, got %v", "Seville", updated.City)
	}
}

// PR-10: Update non-existent profile returns ErrProfileNotFound.
func TestProfileRepository_Update_ProfileNotFound(t *testing.T) {
	truncate(t)
	userID := createTestUser(t, "Ivan")
	repo := NewProfileRepository(testDB)

	// Do NOT call EnsureExists — no profile exists.
	_, err := repo.Update(context.Background(), userID, profile.ProfilePatch{
		DisplayName: ptr("Ivan Updated"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent profile, got nil")
	}
	if !errors.Is(err, ckerrors.ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got: %v", err)
	}
}

// ---- ProfileRepository: List ----

// createProfilesForList inserts N users with profiles for list tests.
// Display names are generated so they sort predictably: "User 01", "User 02", …
func createProfilesForList(t *testing.T, n int) {
	t.Helper()
	repo := NewProfileRepository(testDB)
	userRepo := NewUserRepository(testDB)

	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("User %02d", i)
		u, err := userRepo.Upsert(context.Background(), &user.User{
			Email:      fmt.Sprintf("list-user-%d@example.com", i),
			Name:       name,
			Role:       user.RoleUser,
			Provider:   user.ProviderGoogle,
			ProviderID: fmt.Sprintf("google-list-%d", i),
		})
		if err != nil {
			t.Fatalf("Upsert user %d: %v", i, err)
		}
		if err := repo.EnsureExists(context.Background(), u.ID, name); err != nil {
			t.Fatalf("EnsureExists user %d: %v", i, err)
		}
	}
}

// PR-11: Returns all profiles ordered by display_name ASC.
func TestProfileRepository_List_OrderedByDisplayName(t *testing.T) {
	truncate(t)
	createProfilesForList(t, 3)
	repo := NewProfileRepository(testDB)

	items, total, err := repo.List(context.Background(), profile.ListParams{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total: got %d, want 3", total)
	}
	if len(items) != 3 {
		t.Fatalf("items length: got %d, want 3", len(items))
	}
	// Verify ordering: "User 01" < "User 02" < "User 03"
	for i := 0; i < len(items)-1; i++ {
		if *items[i].DisplayName >= *items[i+1].DisplayName {
			t.Errorf("ordering violation at index %d: %q >= %q",
				i, *items[i].DisplayName, *items[i+1].DisplayName)
		}
	}
}

// PR-12: Pagination — first page of 2, 5 profiles in DB.
func TestProfileRepository_List_FirstPage(t *testing.T) {
	truncate(t)
	createProfilesForList(t, 5)
	repo := NewProfileRepository(testDB)

	items, total, err := repo.List(context.Background(), profile.ListParams{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(items) != 2 {
		t.Errorf("items length: got %d, want 2", len(items))
	}
}

// PR-13: Pagination — last page (page 3 of 3), 5 profiles in DB.
func TestProfileRepository_List_LastPage(t *testing.T) {
	truncate(t)
	createProfilesForList(t, 5)
	repo := NewProfileRepository(testDB)

	items, total, err := repo.List(context.Background(), profile.ListParams{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(items) != 1 {
		t.Errorf("items length: got %d, want 1", len(items))
	}
}

// PR-14: Page beyond total returns empty slice but correct total.
func TestProfileRepository_List_PageBeyondTotal(t *testing.T) {
	truncate(t)
	createProfilesForList(t, 5)
	repo := NewProfileRepository(testDB)

	items, total, err := repo.List(context.Background(), profile.ListParams{Page: 10, PageSize: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(items))
	}
}
