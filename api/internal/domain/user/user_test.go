package user

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected bool
	}{
		{
			name:     "admin role returns true",
			role:     RoleAdmin,
			expected: true,
		},
		{
			name:     "user role returns false",
			role:     RoleUser,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{
				ID:         uuid.New(),
				Email:      "test@example.com",
				Name:       "Test User",
				Role:       tt.role,
				Provider:   ProviderGoogle,
				ProviderID: "google-123",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			assert.Equal(t, tt.expected, u.IsAdmin())
		})
	}
}

func TestRole_Constants(t *testing.T) {
	assert.Equal(t, Role("admin"), RoleAdmin)
	assert.Equal(t, Role("user"), RoleUser)
}

func TestProvider_Constants(t *testing.T) {
	assert.Equal(t, Provider("google"), ProviderGoogle)
	assert.Equal(t, Provider("github"), ProviderGitHub)
	assert.Equal(t, Provider("pat"), ProviderPAT)
}
