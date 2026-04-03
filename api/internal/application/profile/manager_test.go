package profile

import (
	"context"
	"errors"
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

// ---- Update — validation ----

// PM-05: No location fields — skips validation, calls repo.
func TestProfileManager_Update_NoLocationFields_SkipsValidation_CallsRepo(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	patch := profile.ProfilePatch{DisplayName: ptr("X")}
	updated := &profile.Profile{UserID: userID, DisplayName: "X", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	repo.On("Update", mock.Anything, userID, patch).Return(updated, nil)

	mgr := newTestManager(repo)
	got, err := mgr.Update(context.Background(), userID, patch)

	require.NoError(t, err)
	assert.Equal(t, updated, got)
	repo.AssertExpectations(t)
}

// PM-06: Valid country + valid region — calls repo.
func TestProfileManager_Update_ValidCountryAndRegion_CallsRepo(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	patch := profile.ProfilePatch{Country: ptr("ES"), Region: ptr("ES-MD")}
	updated := &profile.Profile{UserID: userID, DisplayName: "Alice", Country: ptr("ES"), Region: ptr("ES-MD"), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	repo.On("Update", mock.Anything, userID, patch).Return(updated, nil)

	mgr := newTestManager(repo)
	got, err := mgr.Update(context.Background(), userID, patch)

	require.NoError(t, err)
	assert.Equal(t, updated, got)
	repo.AssertExpectations(t)
}

// PM-07: Invalid country — returns error before calling repo.
func TestProfileManager_Update_InvalidCountry_ReturnsErrorWithoutCallingRepo(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	patch := profile.ProfilePatch{Country: ptr("XX")}

	mgr := newTestManager(repo)
	_, err := mgr.Update(context.Background(), userID, patch)

	require.Error(t, err)
	repo.AssertNotCalled(t, "Update")
}

// PM-08: Region without country — returns error before calling repo.
func TestProfileManager_Update_RegionWithoutCountry_ReturnsErrorWithoutCallingRepo(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	patch := profile.ProfilePatch{Region: ptr("ES-MD")}

	mgr := newTestManager(repo)
	_, err := mgr.Update(context.Background(), userID, patch)

	require.Error(t, err)
	repo.AssertNotCalled(t, "Update")
}

// PM-09: Valid country, mismatched region — returns error before calling repo.
func TestProfileManager_Update_MismatchedRegion_ReturnsErrorWithoutCallingRepo(t *testing.T) {
	repo := &mockProfileRepository{}
	userID := uuid.New()
	patch := profile.ProfilePatch{Country: ptr("ES"), Region: ptr("FR-75")}

	mgr := newTestManager(repo)
	_, err := mgr.Update(context.Background(), userID, patch)

	require.Error(t, err)
	repo.AssertNotCalled(t, "Update")
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
