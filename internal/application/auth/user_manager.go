// Package auth provides the application-layer orchestration for authentication.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// UserManager handles all user and PAT business logic.
// It is the only application-layer component that talks to UserRepository and PATRepository.
type UserManager interface {
	ResolveByOAuth(ctx context.Context, provider user.Provider, providerID, email, name string) (*user.User, error)
	ResolveByPAT(ctx context.Context, rawPAT string) (*user.User, error)
	CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (string, error)
	RevokePAT(ctx context.Context, id uuid.UUID) error
	ListPATs(ctx context.Context) ([]*pat.PAT, error)
	BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error)
}

type userManager struct {
	users user.UserRepository
	pats  pat.PATRepository
}

// NewUserManager returns a UserManager backed by the given repositories.
func NewUserManager(users user.UserRepository, pats pat.PATRepository) UserManager {
	return &userManager{users: users, pats: pats}
}

func (m *userManager) ResolveByOAuth(
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
		return nil, fmt.Errorf("user manager: ResolveByOAuth: %w", err)
	}
	return result, nil
}

func (m *userManager) ResolveByPAT(ctx context.Context, rawPAT string) (*user.User, error) {
	pats, err := m.pats.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("user manager: ResolveByPAT: %w", err)
	}

	var matched *pat.PAT
	for _, p := range pats {
		if hashKey(rawPAT, p.Salt) == p.KeyHash {
			matched = p
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("user manager: ResolveByPAT: %w", ckerrors.ErrInvalidPAT)
	}
	if matched.IsExpired() {
		return nil, fmt.Errorf("user manager: ResolveByPAT: %w", ckerrors.ErrPATExpired)
	}

	u, err := m.users.FindByProvider(ctx, user.ProviderPAT, matched.ID.String())
	if err != nil {
		return nil, fmt.Errorf("user manager: ResolveByPAT: %w", err)
	}
	return u, nil
}

func (m *userManager) CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (string, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: generate key: %w", err)
	}
	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: generate salt: %w", err)
	}

	saved, err := m.pats.Save(ctx, &pat.PAT{
		KeyHash:   hashKey(rawKey, salt),
		Salt:      salt,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: save: %w", err)
	}

	existing, err := m.users.FindByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: find user: %w", err)
	}
	existing.ProviderID = saved.ID.String()
	if _, err := m.users.Upsert(ctx, existing); err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: link user: %w", err)
	}

	return rawKey, nil
}

func (m *userManager) RevokePAT(ctx context.Context, id uuid.UUID) error {
	if err := m.pats.Delete(ctx, id); err != nil {
		return fmt.Errorf("user manager: RevokePAT: %w", err)
	}
	return nil
}

func (m *userManager) ListPATs(ctx context.Context) ([]*pat.PAT, error) {
	pats, err := m.pats.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("user manager: ListPATs: %w", err)
	}
	return pats, nil
}

func (m *userManager) BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (string, error) {
	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("user manager: BootstrapAdmin: generate salt: %w", err)
	}

	savedPAT, err := m.pats.Save(ctx, &pat.PAT{
		KeyHash: hashKey(rawPAT, salt),
		Salt:    salt,
	})
	if err != nil {
		return "", fmt.Errorf("user manager: BootstrapAdmin: save PAT: %w", err)
	}

	if _, err := m.users.Upsert(ctx, &user.User{
		Email:      email,
		Name:       name,
		Role:       user.RoleAdmin,
		Provider:   user.ProviderPAT,
		ProviderID: savedPAT.ID.String(),
	}); err != nil {
		return "", fmt.Errorf("user manager: BootstrapAdmin: upsert user: %w", err)
	}

	return rawPAT, nil
}
