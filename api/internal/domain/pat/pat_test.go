package pat

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPAT_IsExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "no expiry is never expired",
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "future expiry is not expired",
			expiresAt: &future,
			expected:  false,
		},
		{
			name:      "past expiry is expired",
			expiresAt: &past,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PAT{
				ID:        uuid.New(),
				KeyHash:   "somehash",
				Salt:      "somesalt",
				ExpiresAt: tt.expiresAt,
				CreatedAt: time.Now(),
			}
			assert.Equal(t, tt.expected, p.IsExpired())
		})
	}
}
