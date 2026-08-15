package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/courtknights/courtknights/internal/domain/pat"
	domainprofile "github.com/courtknights/courtknights/internal/domain/profile"
	"github.com/courtknights/courtknights/internal/domain/user"
)

// mockUserRepository is a testify mock for user.UserRepository.
type mockUserRepository struct{ mock.Mock }

func (m *mockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *mockUserRepository) FindByProvider(ctx context.Context, provider user.Provider, providerID string) (*user.User, error) {
	args := m.Called(ctx, provider, providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *mockUserRepository) Upsert(ctx context.Context, u *user.User) (*user.User, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *mockUserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role user.Role) error {
	return m.Called(ctx, id, role).Error(0)
}

func (m *mockUserRepository) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// mockPATRepository is a testify mock for pat.PATRepository.
type mockPATRepository struct{ mock.Mock }

func (m *mockPATRepository) FindAll(ctx context.Context) ([]*pat.PAT, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*pat.PAT), args.Error(1)
}

func (m *mockPATRepository) FindByID(ctx context.Context, id uuid.UUID) (*pat.PAT, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pat.PAT), args.Error(1)
}

func (m *mockPATRepository) Save(ctx context.Context, p *pat.PAT) (*pat.PAT, error) {
	args := m.Called(ctx, p)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pat.PAT), args.Error(1)
}

func (m *mockPATRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// mockProfileManager is a testify mock satisfying ProfileInitializer (and, for
// convenience, the full application/profile.ProfileManager method set).
type mockProfileManager struct{ mock.Mock }

func (m *mockProfileManager) EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error {
	return m.Called(ctx, userID, displayName).Error(0)
}

func (m *mockProfileManager) GetByUserID(ctx context.Context, userID uuid.UUID) (*domainprofile.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainprofile.Profile), args.Error(1)
}

func (m *mockProfileManager) Update(ctx context.Context, userID uuid.UUID, patch domainprofile.ProfilePatch) (*domainprofile.Profile, error) {
	args := m.Called(ctx, userID, patch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainprofile.Profile), args.Error(1)
}

func (m *mockProfileManager) List(ctx context.Context, params domainprofile.ListParams) ([]*domainprofile.ProfileListItem, int64, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domainprofile.ProfileListItem), args.Get(1).(int64), args.Error(2)
}
