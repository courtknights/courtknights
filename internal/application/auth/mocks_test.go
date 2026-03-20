package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/courtknights/courtknights/internal/domain/pat"
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
