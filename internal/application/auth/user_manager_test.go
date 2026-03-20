package auth

import (
	"context"
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

func newUserManager(users *mockUserRepository, pats *mockPATRepository) *UserManager {
	return NewUserManager(users, pats)
}

// ---- ResolveByOAuth ----

func TestManager_ResolveByOAuth_CreatesUserOnFirstCall(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManager(users, pats)

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
	mgr := newUserManager(users, pats)

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
	mgr := newUserManager(users, pats)

	rawKey := "correct-raw-key"
	salt, _ := generateSalt()
	hash := hashKey(rawKey, salt)
	patID := uuid.New()

	storedPAT := &pat.PAT{ID: patID, KeyHash: hash, Salt: salt}
	linkedUser := &user.User{ID: uuid.New(), Email: "carol@example.com", Role: user.RoleUser}

	pats.On("FindAll", context.Background()).Return([]*pat.PAT{storedPAT}, nil)
	users.On("FindByProvider", context.Background(), user.ProviderPAT, patID.String()).Return(linkedUser, nil)

	got, err := mgr.ResolveByPAT(context.Background(), rawKey)
	require.NoError(t, err)
	assert.Equal(t, linkedUser.ID, got.ID)
}

func TestManager_ResolveByPAT_ReturnsErrInvalidPAT_WhenNoMatch(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManager(users, pats)

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
	mgr := newUserManager(users, pats)

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

func TestManager_CreatePAT_ReturnsNonEmptyRawKey(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManager(users, pats)

	userID := uuid.New()
	patID := uuid.New()
	existingUser := &user.User{ID: userID, Email: "dave@example.com", Provider: user.ProviderPAT, ProviderID: "old"}

	pats.On("Save", context.Background(), mock.AnythingOfType("*pat.PAT")).
		Return(&pat.PAT{ID: patID, KeyHash: "h", Salt: "s"}, nil)
	users.On("FindByID", context.Background(), userID).Return(existingUser, nil)
	users.On("Upsert", context.Background(), mock.AnythingOfType("*user.User")).
		Return(existingUser, nil)

	rawKey, err := mgr.CreatePAT(context.Background(), userID, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, rawKey)
}

func TestManager_CreatePAT_StoredHashDiffersFromRawKey(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManager(users, pats)

	userID := uuid.New()
	patID := uuid.New()
	existingUser := &user.User{ID: userID, Email: "eve@example.com", Provider: user.ProviderPAT}

	var capturedPAT *pat.PAT
	pats.On("Save", context.Background(), mock.AnythingOfType("*pat.PAT")).
		Run(func(args mock.Arguments) { capturedPAT = args.Get(1).(*pat.PAT) }).
		Return(&pat.PAT{ID: patID, KeyHash: "h", Salt: "s"}, nil)
	users.On("FindByID", context.Background(), userID).Return(existingUser, nil)
	users.On("Upsert", context.Background(), mock.AnythingOfType("*user.User")).Return(existingUser, nil)

	rawKey, err := mgr.CreatePAT(context.Background(), userID, nil)
	require.NoError(t, err)
	assert.NotEqual(t, rawKey, capturedPAT.KeyHash, "stored hash must differ from raw key")
}

// ---- BootstrapAdmin ----

func TestManager_BootstrapAdmin_CreatesAdminAndPAT(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManager(users, pats)

	patID := uuid.New()
	pats.On("Save", context.Background(), mock.AnythingOfType("*pat.PAT")).
		Return(&pat.PAT{ID: patID}, nil)
	users.On("Upsert", context.Background(), mock.AnythingOfType("*user.User")).
		Return(&user.User{ID: uuid.New(), Role: user.RoleAdmin}, nil)

	rawPAT, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)
	assert.Equal(t, "bootstrap-secret", rawPAT)
	users.AssertExpectations(t)
	pats.AssertExpectations(t)
}

func TestManager_RevokePAT_CallsDelete(t *testing.T) {
	users := &mockUserRepository{}
	pats := &mockPATRepository{}
	mgr := newUserManager(users, pats)

	patID := uuid.New()
	pats.On("Delete", context.Background(), patID).Return(nil)

	err := mgr.RevokePAT(context.Background(), patID)
	require.NoError(t, err)
	pats.AssertExpectations(t)
}
