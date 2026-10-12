package profile

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/profile"
)

// mockProfileRepository is a testify mock for profile.ProfileRepository.
type mockProfileRepository struct{ mock.Mock }

func (m *mockProfileRepository) EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error {
	return m.Called(ctx, userID, displayName).Error(0)
}

func (m *mockProfileRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*profile.Profile), args.Error(1)
}

func (m *mockProfileRepository) Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error) {
	args := m.Called(ctx, userID, patch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*profile.Profile), args.Error(1)
}

func (m *mockProfileRepository) List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*profile.ProfileListItem), args.Get(1).(int64), args.Error(2)
}

// helpers

func ptr[T any](v T) *T { return &v }

func newTestManager(repo *mockProfileRepository) ProfileManager {
	return NewProfileManager(repo)
}

// ---- EnsureExists ----

// PM-01: Delegates to repository successfully.
func TestProfileManager_EnsureExists_DelegatesToRepo_ReturnsNil(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("EnsureExists", mock.Anything, userID, "Alice").Return(nil)

	mgr := newTestManager(repo)
	err := mgr.EnsureExists(context.Background(), userID, "Alice")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// PM-02: Propagates repository error.
func TestProfileManager_EnsureExists_RepoError_PropagatesError(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repoErr := errors.New("db error")
	repo.On("EnsureExists", mock.Anything, userID, "Alice").Return(repoErr)

	mgr := newTestManager(repo)
	err := mgr.EnsureExists(context.Background(), userID, "Alice")

	require.Error(t, err)
	assert.ErrorIs(t, err, repoErr)
	repo.AssertExpectations(t)
}

// ---- GetByUserID ----

// PM-03: Returns profile from repository.
func TestProfileManager_GetByUserID_ReturnsProfile(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	expected := &profile.Profile{UserID: userID, DisplayName: "Alice", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	repo.On("FindByUserID", mock.Anything, userID).Return(expected, nil)

	mgr := newTestManager(repo)
	got, err := mgr.GetByUserID(context.Background(), userID)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	repo.AssertExpectations(t)
}

// PM-04: Propagates ErrProfileNotFound.
func TestProfileManager_GetByUserID_ProfileNotFound_PropagatesError(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID).Return(nil, ckerrors.ErrProfileNotFound)

	mgr := newTestManager(repo)
	_, err := mgr.GetByUserID(context.Background(), userID)

	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrProfileNotFound)
	repo.AssertExpectations(t)
}

// ---- Update ----

// storedProfile returns a profile with the given country and region.
func storedProfile(userID uuid.UUID, country, region *string) *profile.Profile {
	return &profile.Profile{UserID: userID, DisplayName: "Alice", Country: country, Region: region}
}

// TestProfileManager_Update_LocationRules covers every row of the Location
// rules table in FEATURE_clearable_profile_fields/02_architecture.md.
func TestProfileManager_Update_LocationRules(t *testing.T) {
	es, esmd := ptr("ES"), ptr("ES-MD")
	tests := []struct {
		name            string
		storedCountry   *string
		storedRegion    *string
		patch           profile.ProfilePatch
		wantInvalid     bool
		wantRepoCountry profile.Field[string]
		wantRepoRegion  profile.Field[string]
	}{
		{
			name: "country changed, region absent → region cleared", storedCountry: es, storedRegion: esmd,
			patch:           profile.ProfilePatch{Country: profile.SetTo("FR")},
			wantRepoCountry: profile.SetTo("FR"), wantRepoRegion: profile.Cleared[string](),
		},
		{
			name: "country and region changed together", storedCountry: es, storedRegion: esmd,
			patch:           profile.ProfilePatch{Country: profile.SetTo("FR"), Region: profile.SetTo("FR-IDF")},
			wantRepoCountry: profile.SetTo("FR"), wantRepoRegion: profile.SetTo("FR-IDF"),
		},
		{
			name: "same country, region absent → region kept", storedCountry: es, storedRegion: esmd,
			patch:           profile.ProfilePatch{Country: profile.SetTo("ES")},
			wantRepoCountry: profile.SetTo("ES"), wantRepoRegion: profile.Unchanged[string](),
		},
		{
			name: "country cleared → region cleared", storedCountry: es, storedRegion: esmd,
			patch:           profile.ProfilePatch{Country: profile.Cleared[string]()},
			wantRepoCountry: profile.Cleared[string](), wantRepoRegion: profile.Cleared[string](),
		},
		{
			name: "region alone, checked against the stored country", storedCountry: es, storedRegion: esmd,
			patch:           profile.ProfilePatch{Region: profile.SetTo("ES-CT")},
			wantRepoCountry: profile.Unchanged[string](), wantRepoRegion: profile.SetTo("ES-CT"),
		},
		{
			name: "region alone from another country", storedCountry: es,
			patch:       profile.ProfilePatch{Region: profile.SetTo("FR-IDF")},
			wantInvalid: true,
		},
		{
			name:        "region alone without a stored country",
			patch:       profile.ProfilePatch{Region: profile.SetTo("ES-MD")},
			wantInvalid: true,
		},
		{
			name: "region cleared", storedCountry: es, storedRegion: esmd,
			patch:           profile.ProfilePatch{Region: profile.Cleared[string]()},
			wantRepoCountry: profile.Unchanged[string](), wantRepoRegion: profile.Cleared[string](),
		},
		{
			name: "country cleared with a region in the same patch", storedCountry: es, storedRegion: esmd,
			patch:       profile.ProfilePatch{Country: profile.Cleared[string](), Region: profile.SetTo("ES-MD")},
			wantInvalid: true,
		},
		{
			name:            "first country on an empty profile",
			patch:           profile.ProfilePatch{Country: profile.SetTo("ES")},
			wantRepoCountry: profile.SetTo("ES"), wantRepoRegion: profile.Cleared[string](),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockProfileRepository{}
			userID := uuid.New()
			repo.On("FindByUserID", mock.Anything, userID).Return(storedProfile(userID, tt.storedCountry, tt.storedRegion), nil)
			updated := &profile.Profile{UserID: userID}
			repo.On("Update", mock.Anything, userID, mock.Anything).Return(updated, nil)

			got, err := newTestManager(repo).Update(context.Background(), userID, tt.patch)

			if tt.wantInvalid {
				require.ErrorIs(t, err, ckerrors.ErrInvalidLocation)
				repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
			assert.Same(t, updated, got)
			sent := repo.Calls[1].Arguments.Get(2).(profile.ProfilePatch)
			assert.Equal(t, tt.wantRepoCountry, sent.Country, "country sent to the repository")
			assert.Equal(t, tt.wantRepoRegion, sent.Region, "region sent to the repository")
		})
	}
}

func TestProfileManager_Update_InvalidCountry(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID).Return(storedProfile(userID, nil, nil), nil)

	_, err := newTestManager(repo).Update(context.Background(), userID, profile.ProfilePatch{Country: profile.SetTo("XX")})

	require.ErrorIs(t, err, ckerrors.ErrInvalidLocation)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestProfileManager_Update_EmptyPatch(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	stored := storedProfile(userID, ptr("ES"), ptr("ES-MD"))
	repo.On("FindByUserID", mock.Anything, userID).Return(stored, nil)

	got, err := newTestManager(repo).Update(context.Background(), userID, profile.ProfilePatch{})

	require.NoError(t, err)
	assert.Same(t, stored, got)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestProfileManager_Update_ProfileNotFound(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID).
		Return(nil, fmt.Errorf("postgres: FindByUserID profile: %w", ckerrors.ErrProfileNotFound))

	_, err := newTestManager(repo).Update(context.Background(), userID, profile.ProfilePatch{City: profile.SetTo("Madrid")})

	require.ErrorIs(t, err, ckerrors.ErrProfileNotFound)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestProfileManager_Update_Conflict(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID).Return(storedProfile(userID, ptr("ES"), nil), nil)
	repo.On("Update", mock.Anything, userID, mock.Anything).
		Return(nil, fmt.Errorf("postgres: Update profile: %w", ckerrors.ErrProfileConflict))

	_, err := newTestManager(repo).Update(context.Background(), userID, profile.ProfilePatch{Region: profile.SetTo("ES-CT")})

	require.ErrorIs(t, err, ckerrors.ErrProfileConflict)
}

func TestProfileManager_Update_RepositoryError(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID).Return(storedProfile(userID, nil, nil), nil)
	repo.On("Update", mock.Anything, userID, mock.Anything).Return(nil, errors.New("db down"))

	_, err := newTestManager(repo).Update(context.Background(), userID, profile.ProfilePatch{City: profile.SetTo("Madrid")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile manager: Update")
}

func TestProfileManager_Update_NonLocationFieldsPassThrough(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	repo.On("FindByUserID", mock.Anything, userID).Return(storedProfile(userID, ptr("ES"), ptr("ES-MD")), nil)
	patch := profile.ProfilePatch{
		DisplayName: ptr("Alice B."),
		City:        profile.Cleared[string](),
		Gender:      profile.Cleared[profile.Gender](),
		DateOfBirth: profile.Cleared[time.Time](),
		Category:    profile.SetTo(profile.CategorySecond),
		Preferences: profile.PreferencesPatch{Handedness: profile.Cleared[profile.Handedness]()},
	}
	repo.On("Update", mock.Anything, userID, patch).Return(&profile.Profile{UserID: userID}, nil)

	_, err := newTestManager(repo).Update(context.Background(), userID, patch)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// ---- List ----

// PM-10: Delegates params to repository.
func TestProfileManager_List_DelegatesToRepo_ReturnsItems(t *testing.T) {
	repo := &mockProfileRepository{}
	params := profile.ListParams{Page: 1, PageSize: 20}
	items := []*profile.ProfileListItem{
		{ID: func() *uuid.UUID { id := uuid.New(); return &id }(), DisplayName: ptr("Alice")},
		{ID: func() *uuid.UUID { id := uuid.New(); return &id }(), DisplayName: ptr("Bob")},
	}
	repo.On("List", mock.Anything, params).Return(items, int64(2), nil)

	mgr := newTestManager(repo)
	got, total, err := mgr.List(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, int64(2), total)
	repo.AssertExpectations(t)
}

// PM-11: Propagates repository error.
func TestProfileManager_List_RepoError_PropagatesError(t *testing.T) {
	repo := &mockProfileRepository{}
	params := profile.ListParams{Page: 1, PageSize: 20}
	repoErr := errors.New("list db error")
	repo.On("List", mock.Anything, params).Return(nil, int64(0), repoErr)

	mgr := newTestManager(repo)
	_, _, err := mgr.List(context.Background(), params)

	require.Error(t, err)
	assert.ErrorIs(t, err, repoErr)
	repo.AssertExpectations(t)
}
