// Package auth provides the application-layer orchestration for authentication.
package auth

import (
	"context"
	"errors"
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
	BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (*user.User, string, error)
	UpdateUserRole(ctx context.Context, userID uuid.UUID, role user.Role) error
	// FindByID returns the current user record. Returns ckerrors.ErrUserNotFound if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*user.User, error)
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
	if errors.Is(err, ckerrors.ErrEmailAlreadyRegistered) {
		return nil, fmt.Errorf("user manager: ResolveByOAuth: %w", m.emailOwnerError(ctx, email))
	}
	if err != nil {
		return nil, fmt.Errorf("user manager: ResolveByOAuth: %w", err)
	}
	return result, nil
}

// emailOwnerError reports which provider already owns email, so the user can
// be told how to sign in (#132). Naming the provider is safe because the
// requester has just proven control of the email with a provider that
// verified it (#131). The provider is left out for "pat" accounts (the
// bootstrap admin): there is no sign-in button for it, and naming it would
// only reveal an administrative account.
func (m *userManager) emailOwnerError(ctx context.Context, email string) error {
	owner, err := m.users.FindByEmail(ctx, email)
	if err != nil || owner.Provider == user.ProviderPAT {
		return &ckerrors.EmailAlreadyRegisteredError{}
	}
	return &ckerrors.EmailAlreadyRegisteredError{Provider: string(owner.Provider)}
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

	u, err := m.users.FindByID(ctx, matched.UserID)
	if err != nil {
		return nil, fmt.Errorf("user manager: ResolveByPAT: %w", err)
	}
	return u, nil
}

// CreatePAT links a new PAT to the user through personal_access_tokens.user_id.
// The user's provider and provider_id are never modified, so OAuth users keep
// signing in with their provider (#136). Returns a wrapped
// ckerrors.ErrUserNotFound, and saves nothing, when the user does not exist.
func (m *userManager) CreatePAT(ctx context.Context, userID uuid.UUID, expiresAt *time.Time) (string, error) {
	if _, err := m.users.FindByID(ctx, userID); err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: find user: %w", err)
	}

	rawKey, err := generateRawKey()
	if err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: generate key: %w", err)
	}
	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: generate salt: %w", err)
	}

	if _, err := m.pats.Save(ctx, &pat.PAT{
		UserID:    userID,
		KeyHash:   hashKey(rawKey, salt),
		Salt:      salt,
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", fmt.Errorf("user manager: CreatePAT: save: %w", err)
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

func (m *userManager) UpdateUserRole(ctx context.Context, userID uuid.UUID, role user.Role) error {
	if err := m.users.UpdateRole(ctx, userID, role); err != nil {
		return fmt.Errorf("user manager: UpdateUserRole: %w", err)
	}
	return nil
}

func (m *userManager) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	u, err := m.users.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user manager: FindByID: %w", err)
	}
	return u, nil
}

func (m *userManager) BootstrapAdmin(ctx context.Context, email, name, rawPAT string) (*user.User, string, error) {
	count, err := m.users.CountAll(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("user manager: BootstrapAdmin: count users: %w", err)
	}
	if count > 0 {
		return nil, "", nil
	}

	salt, err := generateSalt()
	if err != nil {
		return nil, "", fmt.Errorf("user manager: BootstrapAdmin: generate salt: %w", err)
	}

	// The bootstrap user has no external identity: provider_id is an opaque,
	// unique placeholder. The PAT references the user through user_id.
	u, err := m.users.Upsert(ctx, &user.User{
		Email:      email,
		Name:       name,
		Role:       user.RoleAdmin,
		Provider:   user.ProviderPAT,
		ProviderID: uuid.NewString(),
	})
	if err != nil {
		return nil, "", fmt.Errorf("user manager: BootstrapAdmin: upsert user: %w", err)
	}

	if _, err := m.pats.Save(ctx, &pat.PAT{
		UserID:  u.ID,
		KeyHash: hashKey(rawPAT, salt),
		Salt:    salt,
	}); err != nil {
		return nil, "", fmt.Errorf("user manager: BootstrapAdmin: save PAT: %w", err)
	}

	return u, rawPAT, nil
}
