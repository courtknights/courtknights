package main

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// Every documented COURTKNIGHTS_* variable must reach its config field.
func TestLoadConfig_ReadsEnvironmentVariables(t *testing.T) {
	tests := []struct {
		env  string
		get  func(Config) string
		want string
	}{
		{"COURTKNIGHTS_SERVER_PORT", func(c Config) string { return c.Server.Port }, "9090"},
		{"COURTKNIGHTS_DATABASE_URL", func(c Config) string { return c.Database.URL }, "postgres://env/db"},
		{"COURTKNIGHTS_JWT_SECRET", func(c Config) string { return c.JWT.Secret }, "env-secret"},
		{"COURTKNIGHTS_JWT_EXPIRY", func(c Config) string { return c.JWT.Expiry.String() }, (2 * time.Hour).String()},
		{"COURTKNIGHTS_GOOGLE_CLIENT_ID", func(c Config) string { return c.Google.ClientID }, "g-id"},
		{"COURTKNIGHTS_GOOGLE_CLIENT_SECRET", func(c Config) string { return c.Google.ClientSecret }, "g-secret"},
		{"COURTKNIGHTS_GOOGLE_REDIRECT_URL", func(c Config) string { return c.Google.RedirectURL }, "http://g/redirect"},
		{"COURTKNIGHTS_GOOGLE_AUTH_URL", func(c Config) string { return c.Google.AuthURL }, "http://g/auth"},
		{"COURTKNIGHTS_GOOGLE_TOKEN_URL", func(c Config) string { return c.Google.TokenURL }, "http://g/token"},
		{"COURTKNIGHTS_GOOGLE_DEVICE_AUTH_URL", func(c Config) string { return c.Google.DeviceAuthURL }, "http://g/device"},
		{"COURTKNIGHTS_GOOGLE_USERINFO_URL", func(c Config) string { return c.Google.UserInfoURL }, "http://g/userinfo"},
		{"COURTKNIGHTS_GITHUB_CLIENT_ID", func(c Config) string { return c.GitHub.ClientID }, "gh-id"},
		{"COURTKNIGHTS_GITHUB_CLIENT_SECRET", func(c Config) string { return c.GitHub.ClientSecret }, "gh-secret"},
		{"COURTKNIGHTS_GITHUB_REDIRECT_URL", func(c Config) string { return c.GitHub.RedirectURL }, "http://gh/redirect"},
		{"COURTKNIGHTS_GITHUB_AUTH_URL", func(c Config) string { return c.GitHub.AuthURL }, "http://gh/auth"},
		{"COURTKNIGHTS_GITHUB_TOKEN_URL", func(c Config) string { return c.GitHub.TokenURL }, "http://gh/token"},
		{"COURTKNIGHTS_GITHUB_DEVICE_AUTH_URL", func(c Config) string { return c.GitHub.DeviceAuthURL }, "http://gh/device"},
		{"COURTKNIGHTS_GITHUB_USERINFO_URL", func(c Config) string { return c.GitHub.UserInfoURL }, "http://gh/user"},
		{"COURTKNIGHTS_BOOTSTRAP_EMAIL", func(c Config) string { return c.Bootstrap.Email }, "admin@example.com"},
		{"COURTKNIGHTS_BOOTSTRAP_NAME", func(c Config) string { return c.Bootstrap.Name }, "Admin"},
		{"COURTKNIGHTS_BOOTSTRAP_PAT", func(c Config) string { return c.Bootstrap.PAT }, "bootstrap-pat"},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv(tt.env, tt.want)
			v := newViper()
			registerFlags(&cobra.Command{}, v)

			assert.Equal(t, tt.want, tt.get(loadConfig(v)))
		})
	}
}

// A flag passed on the command line still wins over the environment.
func TestLoadConfig_FlagOverridesEnvironment(t *testing.T) {
	t.Setenv("COURTKNIGHTS_SERVER_PORT", "9090")
	v := newViper()
	cmd := &cobra.Command{}
	registerFlags(cmd, v)
	if err := cmd.Flags().Set("port", "7070"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	assert.Equal(t, "7070", loadConfig(v).Server.Port)
}
