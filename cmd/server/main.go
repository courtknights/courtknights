// Package main is the entry point for the CourtKnights API server.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/courtknights/courtknights/internal/api"
	apiauth "github.com/courtknights/courtknights/internal/api/auth"
	"github.com/courtknights/courtknights/internal/api/common"
	"github.com/courtknights/courtknights/internal/api/pats"
	"github.com/courtknights/courtknights/internal/api/users"
	appauth "github.com/courtknights/courtknights/internal/application/auth"
	domainuser "github.com/courtknights/courtknights/internal/domain/user"
	"github.com/courtknights/courtknights/internal/infrastructure/jwt"
	"github.com/courtknights/courtknights/internal/infrastructure/oauth2"
	"github.com/courtknights/courtknights/internal/infrastructure/postgres"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	v := viper.New()

	cmd := &cobra.Command{
		Use:   "courtknights-api",
		Short: "CourtKnights API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(cmd.Context(), v)
		},
	}

	// Flags — bound to viper so they can also be set via env vars or defaults.
	cmd.Flags().String("port", "8080", "HTTP listen port (COURTKNIGHTS_SERVER_PORT)")
	cmd.Flags().String("db-url", "", "PostgreSQL connection URL (COURTKNIGHTS_DATABASE_URL)")
	cmd.Flags().String("jwt-secret", "", "JWT signing secret (COURTKNIGHTS_JWT_SECRET)")
	cmd.Flags().Duration("jwt-expiry", time.Hour, "JWT token lifetime (COURTKNIGHTS_JWT_EXPIRY)")
	cmd.Flags().String("google-client-id", "", "Google OAuth2 client ID (COURTKNIGHTS_GOOGLE_CLIENT_ID)")
	cmd.Flags().String("google-client-secret", "", "Google OAuth2 client secret (COURTKNIGHTS_GOOGLE_CLIENT_SECRET)")
	cmd.Flags().String("google-redirect-url", "", "Google OAuth2 redirect URL (COURTKNIGHTS_GOOGLE_REDIRECT_URL)")
	cmd.Flags().String("github-client-id", "", "GitHub OAuth2 client ID (COURTKNIGHTS_GITHUB_CLIENT_ID)")
	cmd.Flags().String("github-client-secret", "", "GitHub OAuth2 client secret (COURTKNIGHTS_GITHUB_CLIENT_SECRET)")
	cmd.Flags().String("github-redirect-url", "", "GitHub OAuth2 redirect URL (COURTKNIGHTS_GITHUB_REDIRECT_URL)")
	cmd.Flags().String("bootstrap-email", "", "Admin bootstrap email (COURTKNIGHTS_BOOTSTRAP_EMAIL)")
	cmd.Flags().String("bootstrap-name", "", "Admin bootstrap name (COURTKNIGHTS_BOOTSTRAP_NAME)")
	cmd.Flags().String("bootstrap-pat", "", "Admin bootstrap PAT (COURTKNIGHTS_BOOTSTRAP_PAT)")

	// Bind each flag to its corresponding env var.
	v.SetEnvPrefix("COURTKNIGHTS")
	v.AutomaticEnv()
	_ = v.BindPFlag("server.port", cmd.Flags().Lookup("port"))
	_ = v.BindPFlag("database.url", cmd.Flags().Lookup("db-url"))
	_ = v.BindPFlag("jwt.secret", cmd.Flags().Lookup("jwt-secret"))
	_ = v.BindPFlag("jwt.expiry", cmd.Flags().Lookup("jwt-expiry"))
	_ = v.BindPFlag("google.client_id", cmd.Flags().Lookup("google-client-id"))
	_ = v.BindPFlag("google.client_secret", cmd.Flags().Lookup("google-client-secret"))
	_ = v.BindPFlag("google.redirect_url", cmd.Flags().Lookup("google-redirect-url"))
	_ = v.BindPFlag("github.client_id", cmd.Flags().Lookup("github-client-id"))
	_ = v.BindPFlag("github.client_secret", cmd.Flags().Lookup("github-client-secret"))
	_ = v.BindPFlag("github.redirect_url", cmd.Flags().Lookup("github-redirect-url"))
	_ = v.BindPFlag("bootstrap.email", cmd.Flags().Lookup("bootstrap-email"))
	_ = v.BindPFlag("bootstrap.name", cmd.Flags().Lookup("bootstrap-name"))
	_ = v.BindPFlag("bootstrap.pat", cmd.Flags().Lookup("bootstrap-pat"))

	return cmd
}

func runServer(ctx context.Context, v *viper.Viper) error {
	// ---- database ----
	db, err := pgxpool.New(ctx, v.GetString("database.url"))
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	// ---- repositories ----
	userRepo := postgres.NewUserRepository(db)
	patRepo := postgres.NewPATRepository(db)

	// ---- jwt adapter ----
	jwtAdapter, err := jwt.NewWithConfig(v.GetString("jwt.secret"), v.GetDuration("jwt.expiry"))
	if err != nil {
		return fmt.Errorf("jwt: %w", err)
	}

	// ---- oauth2 providers ----
	providers := buildProviders(v)

	// ---- application managers ----
	userManager := appauth.NewUserManager(userRepo, patRepo)
	jwtManager := appauth.NewJWTManager(jwtAdapter)
	oauthManager := appauth.NewOAuthManager(providers)
	authManager := appauth.NewAuthManager(userManager, jwtManager, oauthManager)

	// ---- bootstrap admin ----
	if err := runBootstrap(ctx, v, authManager); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// ---- router ----
	router := api.New()
	router.MountPublic("/auth", apiauth.NewRoutes(apiauth.NewHandler(authManager)))
	router.MountProtected("/api/v1", common.JWTMiddleware(jwtManager), pats.NewRoutes(pats.NewHandler(authManager)))
	router.MountProtected("/api/v1", common.JWTMiddleware(jwtManager), users.NewRoutes(users.NewHandler(authManager)))

	// ---- start server ----
	addr := fmt.Sprintf(":%s", v.GetString("server.port"))
	log.Printf("starting server on %s", addr)
	if err := router.Echo().Start(addr); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

func buildProviders(v *viper.Viper) map[domainuser.Provider]oauth2.Provider {
	providers := make(map[domainuser.Provider]oauth2.Provider)

	if v.GetString("google.client_id") != "" {
		providers[domainuser.ProviderGoogle] = oauth2.NewGoogle(oauth2.GoogleConfig{
			ClientID:     v.GetString("google.client_id"),
			ClientSecret: v.GetString("google.client_secret"),
			RedirectURL:  v.GetString("google.redirect_url"),
		}, nil)
	}
	if v.GetString("github.client_id") != "" {
		providers[domainuser.ProviderGitHub] = oauth2.NewGitHub(oauth2.GitHubConfig{
			ClientID:     v.GetString("github.client_id"),
			ClientSecret: v.GetString("github.client_secret"),
			RedirectURL:  v.GetString("github.redirect_url"),
		}, nil)
	}
	return providers
}

func runBootstrap(ctx context.Context, v *viper.Viper, mgr appauth.AuthManager) error {
	email := v.GetString("bootstrap.email")
	name := v.GetString("bootstrap.name")
	rawPAT := v.GetString("bootstrap.pat")

	if email == "" || name == "" || rawPAT == "" {
		return nil
	}

	result, err := mgr.BootstrapAdmin(ctx, email, name, rawPAT)
	if err != nil {
		return fmt.Errorf("BootstrapAdmin: %w", err)
	}
	if result == "" {
		log.Println("bootstrap: skipped — users already exist")
	} else {
		log.Println("bootstrap: admin user created")
	}
	return nil
}
