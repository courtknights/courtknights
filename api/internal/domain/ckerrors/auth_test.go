package ckerrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestEmailAlreadyRegisteredError(t *testing.T) {
	tests := []struct {
		name    string
		err     *EmailAlreadyRegisteredError
		wantMsg string
	}{
		{name: "with provider", err: &EmailAlreadyRegisteredError{Provider: "google"}, wantMsg: "email already registered with provider google"},
		{name: "provider withheld", err: &EmailAlreadyRegisteredError{}, wantMsg: "email already registered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			wrapped := fmt.Errorf("context: %w", tt.err)
			if !errors.Is(wrapped, ErrEmailAlreadyRegistered) {
				t.Error("errors.Is(wrapped, ErrEmailAlreadyRegistered) = false")
			}
			var target *EmailAlreadyRegisteredError
			if !errors.As(wrapped, &target) || target.Provider != tt.err.Provider {
				t.Errorf("errors.As did not recover provider %q", tt.err.Provider)
			}
		})
	}
}
