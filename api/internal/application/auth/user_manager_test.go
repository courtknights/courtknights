package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
)

func newUserManagerForTest(users *mockUserRepository, pats *mockPATRepository) UserManager {
	return NewUserManager(users, pats)
}

// ---- ResolveByOAuth ----

func TestManager_ResolveByOAuth_CreatesUserOnFirstCall(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	expected := &user.User{ID: uuid.New(), Email: "alice@example.com", Name: "Alice", Role: user.RoleUser}
	users.On("Upsert", context.Background(), &user.User{
		Email: "alice@example.com", Name: "Alice",
		Role: user.RoleUser, Provider: user.ProviderGoogle, ProviderID: "g-001",
	}).Return(expected, nil)

	got, err := mgr.ResolveByOAuth(context.Background(), user.ProviderGoogle, "g-001", "alice@example.com", "Alice")
	require.NoError(t, err)
	assert.Equal(t, expected.ID, got.ID)
	users.AssertExpectations(t)
}

func TestManager_ResolveByOAuth_ReturnsExistingUserOnSecondCall(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	existing := &user.User{ID: uuid.New(), Email: "bob@example.com", Name: "Bob", Role: user.RoleUser}
	users.On("Upsert", context.Background(), &user.User{
		Email: "bob@example.com", Name: "Bob",
		Role: user.RoleUser, Provider: user.ProviderGitHub, ProviderID: "gh-002",
	}).Return(existing, nil).Twice()

	first, _ := mgr.ResolveByOAuth(context.Background(), user.ProviderGitHub, "gh-002", "bob@example.com", "Bob")
	second, err := mgr.ResolveByOAuth(context.Background(), user.ProviderGitHub, "gh-002", "bob@example.com", "Bob")
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
}

// ---- ResolveByPAT ----

func TestManager_ResolveByPAT_ReturnsUser_WhenKeyMatches(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	rawKey := "correct-raw-key"
	salt, _ := generateSalt()
	hash := hashKey(rawKey, salt)
	linkedUser := &user.User{ID: uuid.New(), Email: "carol@example.com", Role: user.RoleUser}
	storedPAT := &pat.PAT{ID: uuid.New(), UserID: linkedUser.ID, KeyHash: hash, Salt: salt}

	pats.On("FindAll", context.Background()).Return([]*pat.PAT{storedPAT}, nil)
	users.On("FindByID", context.Background(), linkedUser.ID).Return(linkedUser, nil)

	got, err := mgr.ResolveByPAT(context.Background(), rawKey)
	require.NoError(t, err)
	assert.Equal(t, linkedUser.ID, got.ID)
}

func TestManager_ResolveByPAT_ReturnsErrInvalidPAT_WhenNoMatch(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	salt, _ := generateSalt()
	storedPAT := &pat.PAT{ID: uuid.New(), KeyHash: hashKey("real-key", salt), Salt: salt}

	pats.On("FindAll", context.Background()).Return([]*pat.PAT{storedPAT}, nil)

	_, err := mgr.ResolveByPAT(context.Background(), "wrong-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrInvalidPAT)
}

func TestManager_ResolveByPAT_ReturnsErrPATExpired_WhenExpired(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	rawKey := "some-key"
	salt, _ := generateSalt()
	past := time.Now().Add(-1 * time.Hour)
	storedPAT := &pat.PAT{
		ID:        uuid.New(),
		KeyHash:   hashKey(rawKey, salt),
		Salt:      salt,
		ExpiresAt: &past,
	}

	pats.On("FindAll", context.Background()).Return([]*pat.PAT{storedPAT}, nil)

	_, err := mgr.ResolveByPAT(context.Background(), rawKey)
	require.Error(t, err)
	assert.ErrorIs(t, err, ckerrors.ErrPATExpired)
}

// ---- CreatePAT ----

// #136: the PAT references the user through user_id; the user row is never
// rewritten, for OAuth and PAT users alike.
func TestManager_CreatePAT_LinksPATToUserWithoutTouchingIdentity(t *testing.T) {
	tests := []struct {
		name     string
		provider user.Provider
	}{
		{name: "google user", provider: user.ProviderGoogle},
		{name: "github user", provider: user.ProviderGitHub},
		{name: "pat user", provider: user.ProviderPAT},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &mockUserRepository{}
			pats := &mockPATRepository{}
			mgr := newUserManagerForTest(users, pats)

			existing := &user.User{ID: uuid.New(), Email: "dave@example.com", Provider: tt.provider, ProviderID: "provider-id"}
			users.On("FindByID", context.Background(), existing.ID).Return(existing, nil)

			var saved *pat.PAT
			pats.On("Save", context.Background(), mock.AnythingOfType("*pat.PAT")).
				Run(func(args mock.Arguments) { saved = args.Get(1).(*pat.PAT) }).
				Return(&pat.PAT{ID: uuid.New(), UserID: existing.ID}, nil)

			rawKey, err := mgr.CreatePAT(context.Background(), existing.ID, nil)
			require.NoError(t, err)
			assert.NotEmpty(t, rawKey)
			require.NotNil(t, saved)
			assert.Equal(t, existing.ID, saved.UserID)
			assert.NotEqual(t, rawKey, saved.KeyHash, "stored hash must differ from raw key")
			users.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
			assert.Equal(t, tt.provider, existing.Provider)
			assert.Equal(t, "provider-id", existing.ProviderID)
		})
	}
}

// #136: an unknown user is reported as ErrUserNotFound and no PAT is saved.
func TestManager_CreatePAT_UnknownUser_ReturnsErrUserNotFoundAndSavesNothing(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	userID := uuid.New()
	users.On("FindByID", context.Background(), userID).
		Return(nil, fmt.Errorf("postgres: FindByID: %w", ckerrors.ErrUserNotFound))

	rawKey, err := mgr.CreatePAT(context.Background(), userID, nil)
	require.ErrorIs(t, err, ckerrors.ErrUserNotFound)
	assert.Empty(t, rawKey)
	pats.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

// ---- BootstrapAdmin ----

func TestManager_BootstrapAdmin_CreatesAdminAndPAT(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	adminID := uuid.New()
	users.On("CountAll", context.Background()).Return(int64(0), nil)
	users.On("Upsert", context.Background(), mock.MatchedBy(func(u *user.User) bool {
		return u.Role == user.RoleAdmin && u.Provider == user.ProviderPAT && u.ProviderID != ""
	})).Return(&user.User{ID: adminID, Role: user.RoleAdmin}, nil)
	pats.On("Save", context.Background(), mock.MatchedBy(func(p *pat.PAT) bool {
		return p.UserID == adminID
	})).Return(&pat.PAT{ID: uuid.New(), UserID: adminID}, nil)

	u, rawPAT, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)
	assert.Equal(t, "bootstrap-secret", rawPAT)
	assert.NotNil(t, u)
	users.AssertExpectations(t)
	pats.AssertExpectations(t)
}

func TestManager_BootstrapAdmin_IsNoOpWhenUsersExist(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	users.On("CountAll", context.Background()).Return(int64(1), nil)

	u, rawPAT, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)
	assert.Nil(t, u)
	assert.Empty(t, rawPAT)
	// Save and Upsert must NOT be called
	pats.AssertNotCalled(t, "Save")
	users.AssertNotCalled(t, "Upsert")
}

func TestManager_RevokePAT_CallsDelete(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManagerForTest(users, pats)

	patID := uuid.New()
	pats.On("Delete", context.Background(), patID).Return(nil)

	err := mgr.RevokePAT(context.Background(), patID)
	require.NoError(t, err)
	pats.AssertExpectations(t)
}
