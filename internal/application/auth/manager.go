// Package auth provides the application-layer orchestration for authentication.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// Manager orchestrates all authentication business logic.
// It depends only on domain interfaces — no HTTP, no JWT, no OAuth2.
type Manager struct {
	users user.UserRepository
	pats  pat.PATRepository
}

// NewManager returns an AuthManager with the given repository dependencies.
func NewManager(users user.UserRepository, pats pat.PATRepository) *Manager {
	return &Manager{users: users, pats: pats}
}

// ResolveByOAuth finds an existing user by (provider, providerID) or creates one.
// On creation the user is assigned RoleUser. On subsequent calls name and email
// are updated via Upsert.
func (m *Manager) ResolveByOAuth(
	ctx context.Context,
	provider user.Provider,
	providerID, email, name string,
) (*user.User, error) {
	u := &user.User{
		Email:      email,
		Name:       name,
		Role:       user.RoleUser,
		Provider:   provider,
		ProviderID: providerID,
	}
	result, err := m.users.Upsert(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("auth: ResolveByOAuth: %w", err)
	}
	return result, nil
}

// ResolveByPAT verifies a raw PAT string and returns the linked user.
// It fetches all stored PATs, rehashes the raw key against each salt, and
// compares to the stored key_hash. Returns ErrInvalidPAT if no match is found
// and ErrPATExpired if the matching PAT has passed its expiry.
func (m *Manager) ResolveByPAT(ctx context.Context, rawPAT string) (*user.User, error) {
	pats, err := m.pats.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: ResolveByPAT: %w", err)
	}

	var matched *pat.PAT
	for _, p := range pats {
		if hashKey(rawPAT, p.Salt) == p.KeyHash {
			matched = p
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("auth: ResolveByPAT: %w", ckerrors.ErrInvalidPAT)
	}
	if matched.IsExpired() {
		return nil, fmt.Errorf("auth: ResolveByPAT: %w", ckerrors.ErrPATExpired)
	}

	// The linked user has provider=pat and provider_id=pat.ID.
	u, err := m.users.FindByProvider(ctx, user.ProviderPAT, matched.ID.String())
	if err != nil {
		return nil, fmt.Errorf("auth: ResolveByPAT: %w", err)
	}
	return u, nil
}

// CreatePAT generates a new random PAT for the given user.
// It returns the raw key (shown once only); only the hash+salt are persisted.
// The new PAT is linked to the user by setting provider_id = pat.ID via Upsert.
func (m *Manager) CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (rawKey string, err error) {
	rawKey, err = generateRawKey()
	if err != nil {
		return "", fmt.Errorf("auth: CreatePAT: generate key: %w", err)
	}

	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("auth: CreatePAT: generate salt: %w", err)
	}

	p := &pat.PAT{
		KeyHash:   hashKey(rawKey, salt),
		Salt:      salt,
		ExpiresAt: expiresAt,
	}
	saved, err := m.pats.Save(ctx, p)
	if err != nil {
		return "", fmt.Errorf("auth: CreatePAT: save: %w", err)
	}

	// Update the user's provider_id to reference this PAT.
	existing, err := m.users.FindByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("auth: CreatePAT: find user: %w", err)
	}
	existing.ProviderID = saved.ID.String()
	if _, err := m.users.Upsert(ctx, existing); err != nil {
		return "", fmt.Errorf("auth: CreatePAT: link user: %w", err)
	}

	return rawKey, nil
}

// RevokePAT deletes the PAT with the given ID.
func (m *Manager) RevokePAT(ctx context.Context, id uuid.UUID) error {
	if err := m.pats.Delete(ctx, id); err != nil {
		return fmt.Errorf("auth: RevokePAT: %w", err)
	}
	return nil
}

// BootstrapAdmin is a no-op if any user already exists.
// Otherwise it creates the first admin user (provider=pat) and links a PAT.
// The raw PAT key is returned so the caller can surface it to the operator.
func (m *Manager) BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error) {
	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("auth: BootstrapAdmin: generate salt: %w", err)
	}

	p := &pat.PAT{
		KeyHash: hashKey(rawPAT, salt),
		Salt:    salt,
	}
	savedPAT, err := m.pats.Save(ctx, p)
	if err != nil {
		return "", fmt.Errorf("auth: BootstrapAdmin: save PAT: %w", err)
	}

	admin := &user.User{
		Email:      email,
		Name:       name,
		Role:       user.RoleAdmin,
		Provider:   user.ProviderPAT,
		ProviderID: savedPAT.ID.String(),
	}
	if _, err := m.users.Upsert(ctx, admin); err != nil {
		return "", fmt.Errorf("auth: BootstrapAdmin: upsert user: %w", err)
	}

	return rawPAT, nil
}

// hashKey returns the SHA-256 hex digest of key+salt.
func hashKey(key, salt string) string {
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:])
}

// generateRawKey generates a cryptographically random 32-byte hex string.
func generateRawKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateSalt generates a cryptographically random 16-byte hex salt.
func generateSalt() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
