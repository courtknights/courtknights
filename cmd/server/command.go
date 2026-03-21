package main

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newRootCmd builds the root Cobra command, registers all flags, and binds them
// to the provided Viper instance so they can also be set via environment variables.
func newRootCmd() *cobra.Command {
	v := viper.New()
	v.SetEnvPrefix("COURTKNIGHTS")
	v.AutomaticEnv()

	cmd := &cobra.Command{
		Use:   "courtknights-api",
		Short: "CourtKnights API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(cmd.Context(), loadConfig(v))
		},
	}

	registerFlags(cmd, v)

	return cmd
}

// registerFlags declares all CLI flags and binds each one to its Viper key.
func registerFlags(cmd *cobra.Command, v *viper.Viper) {
	f := cmd.Flags()

	// Server
	f.String("port", "8080", "HTTP listen port (COURTKNIGHTS_SERVER_PORT)")
	_ = v.BindPFlag("server.port", f.Lookup("port"))

	// Database
	f.String("db-url", "", "PostgreSQL connection URL (COURTKNIGHTS_DATABASE_URL)")
	_ = v.BindPFlag("database.url", f.Lookup("db-url"))

	// JWT
	f.String("jwt-secret", "", "JWT signing secret (COURTKNIGHTS_JWT_SECRET)")
	_ = v.BindPFlag("jwt.secret", f.Lookup("jwt-secret"))

	f.Duration("jwt-expiry", time.Hour, "JWT token lifetime (COURTKNIGHTS_JWT_EXPIRY)")
	_ = v.BindPFlag("jwt.expiry", f.Lookup("jwt-expiry"))

	// Google OAuth2
	f.String("google-client-id", "", "Google OAuth2 client ID (COURTKNIGHTS_GOOGLE_CLIENT_ID)")
	_ = v.BindPFlag("google.client_id", f.Lookup("google-client-id"))

	f.String("google-client-secret", "", "Google OAuth2 client secret (COURTKNIGHTS_GOOGLE_CLIENT_SECRET)")
	_ = v.BindPFlag("google.client_secret", f.Lookup("google-client-secret"))

	f.String("google-redirect-url", "", "Google OAuth2 redirect URL (COURTKNIGHTS_GOOGLE_REDIRECT_URL)")
	_ = v.BindPFlag("google.redirect_url", f.Lookup("google-redirect-url"))

	// GitHub OAuth2
	f.String("github-client-id", "", "GitHub OAuth2 client ID (COURTKNIGHTS_GITHUB_CLIENT_ID)")
	_ = v.BindPFlag("github.client_id", f.Lookup("github-client-id"))

	f.String("github-client-secret", "", "GitHub OAuth2 client secret (COURTKNIGHTS_GITHUB_CLIENT_SECRET)")
	_ = v.BindPFlag("github.client_secret", f.Lookup("github-client-secret"))

	f.String("github-redirect-url", "", "GitHub OAuth2 redirect URL (COURTKNIGHTS_GITHUB_REDIRECT_URL)")
	_ = v.BindPFlag("github.redirect_url", f.Lookup("github-redirect-url"))

	// Bootstrap admin
	f.String("bootstrap-email", "", "Admin bootstrap email (COURTKNIGHTS_BOOTSTRAP_EMAIL)")
	_ = v.BindPFlag("bootstrap.email", f.Lookup("bootstrap-email"))

	f.String("bootstrap-name", "", "Admin bootstrap name (COURTKNIGHTS_BOOTSTRAP_NAME)")
	_ = v.BindPFlag("bootstrap.name", f.Lookup("bootstrap-name"))

	f.String("bootstrap-pat", "", "Admin bootstrap PAT (COURTKNIGHTS_BOOTSTRAP_PAT)")
	_ = v.BindPFlag("bootstrap.pat", f.Lookup("bootstrap-pat"))
}
