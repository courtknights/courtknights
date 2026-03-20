//go:build integration

package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/infrastructure/postgres"
)

var integrationDB *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("ck_auth_test"),
		tcpostgres.WithUsername("ck"),
		tcpostgres.WithPassword("ck"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get connection string: %v\n", err)
		os.Exit(1)
	}

	integrationDB, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open pool: %v\n", err)
		os.Exit(1)
	}
	defer integrationDB.Close()

	if err := applySchema(ctx, integrationDB); err != nil {
		fmt.Fprintf(os.Stderr, "apply schema: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func applySchema(ctx context.Context, db *pgxpool.Pool) error {
	ddl := `
	CREATE TABLE IF NOT EXISTS users (
		id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
		email       VARCHAR(255) NOT NULL UNIQUE,
		name        VARCHAR(255) NOT NULL,
		role        VARCHAR(50)  NOT NULL DEFAULT 'user'
		                         CHECK (role IN ('admin', 'user')),
		provider    VARCHAR(50)  NOT NULL
		                         CHECK (provider IN ('google', 'github', 'pat')),
		provider_id VARCHAR(255) NOT NULL,
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
		UNIQUE (provider, provider_id)
	);
	CREATE TABLE IF NOT EXISTS personal_access_tokens (
		id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
		key_hash    VARCHAR(255) NOT NULL,
		salt        VARCHAR(255) NOT NULL,
		expires_at  TIMESTAMPTZ,
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
	);`
	_, err := db.Exec(ctx, ddl)
	return err
}

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := integrationDB.Exec(context.Background(), "TRUNCATE users, personal_access_tokens CASCADE")
	require.NoError(t, err, "truncate tables")
}

func newUserManagerForIntegration() auth.UserManager {
	return auth.NewUserManager(
		postgres.NewUserRepository(integrationDB),
		postgres.NewPATRepository(integrationDB),
	)
}

// ---- bootstrap integration tests ----

func TestBootstrapAdmin_EmptyDB_CreatesAdminAndPAT(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()

	rawPAT, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "bootstrap-secret")
	require.NoError(t, err)
	assert.Equal(t, "bootstrap-secret", rawPAT, "raw PAT must be returned on first bootstrap")

	// Verify that exactly one user exists
	var count int64
	err = integrationDB.QueryRow(context.Background(), "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Verify the user has the admin role
	var role string
	err = integrationDB.QueryRow(context.Background(), "SELECT role FROM users WHERE email = $1", "admin@example.com").Scan(&role)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestBootstrapAdmin_NonEmptyDB_IsNoOp(t *testing.T) {
	truncateTables(t)
	mgr := newUserManagerForIntegration()

	// First bootstrap creates the admin
	_, err := mgr.BootstrapAdmin(context.Background(), "admin@example.com", "Admin", "secret-1")
	require.NoError(t, err)

	// Second bootstrap must be a no-op
	result, err := mgr.BootstrapAdmin(context.Background(), "other@example.com", "Other", "secret-2")
	require.NoError(t, err)
	assert.Empty(t, result, "second bootstrap must return empty string")

	// Still exactly one user
	var count int64
	err = integrationDB.QueryRow(context.Background(), "SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "no duplicate users must be created")
}
