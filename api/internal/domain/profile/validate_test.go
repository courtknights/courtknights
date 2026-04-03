package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptr(s string) *string {
	return &s
}

func TestValidateLocation(t *testing.T) {
	tests := []struct {
		id      string
		name    string
		country *string
		region  *string
		wantErr bool
	}{
		{
			id:      "VL-01",
			name:    "both nil — no validation",
			country: nil,
			region:  nil,
			wantErr: false,
		},
		{
			id:      "VL-02",
			name:    "valid country, no region",
			country: ptr("ES"),
			region:  nil,
			wantErr: false,
		},
		{
			id:      "VL-03",
			name:    "valid country + valid matching region",
			country: ptr("ES"),
			region:  ptr("ES-MD"),
			wantErr: false,
		},
		{
			id:      "VL-04",
			name:    "invalid country code",
			country: ptr("XX"),
			region:  nil,
			wantErr: true,
		},
		{
			id:      "VL-05",
			name:    "valid country + region from a different country",
			country: ptr("ES"),
			region:  ptr("FR-75"),
			wantErr: true,
		},
		{
			id:      "VL-06",
			name:    "valid country + invalid region format",
			country: ptr("ES"),
			region:  ptr("INVALID"),
			wantErr: true,
		},
		{
			id:      "VL-07",
			name:    "region provided without country",
			country: nil,
			region:  ptr("ES-MD"),
			wantErr: true,
		},
		{
			id:      "VL-08",
			name:    "empty string country (not nil)",
			country: ptr(""),
			region:  nil,
			wantErr: true,
		},
		{
			id:      "VL-09",
			name:    "empty string region with valid country",
			country: ptr("US"),
			region:  ptr(""),
			wantErr: true,
		},
		{
			id:      "VL-10",
			name:    "case-sensitive country code (lowercase)",
			country: ptr("es"),
			region:  nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.id+"_"+tc.name, func(t *testing.T) {
			err := ValidateLocation(tc.country, tc.region)
			if tc.wantErr {
				assert.Error(t, err, "expected an error but got none")
			} else {
				assert.NoError(t, err, "expected no error but got: %v", err)
			}
		})
	}
}
