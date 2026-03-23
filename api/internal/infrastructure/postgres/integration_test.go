//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/pat"
	"github.com/courtknights/courtknights/internal/domain/user"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("courtknights_test"),
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

	testDB, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open pool: %v\n", err)
		os.Exit(1)
	}
	defer testDB.Close()

	if err := applyMigrations(ctx, testDB); err != nil {
		fmt.Fprintf(os.Stderr, "apply migrations: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func applyMigrations(ctx context.Context, db *pgxpool.Pool) error {
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

func truncate(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), "TRUNCATE users, personal_access_tokens CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// ---- UserRepository ----

func TestUserRepository_FindByProvider_NotFound(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	_, err := repo.FindByProvider(context.Background(), user.ProviderGoogle, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ckerrors.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserRepository_Upsert_CreatesOnFirstCall(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	u := &user.User{
		Email: "alice@example.com", Name: "Alice",
		Role: user.RoleUser, Provider: user.ProviderGoogle, ProviderID: "google-001",
	}
	created, err := repo.Upsert(context.Background(), u)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if created.Email != u.Email {
		t.Errorf("email: got %q, want %q", created.Email, u.Email)
	}
}

func TestUserRepository_Upsert_UpdatesOnConflict(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	u := &user.User{
		Email: "bob@example.com", Name: "Bob",
		Role: user.RoleUser, Provider: user.ProviderGitHub, ProviderID: "github-002",
	}
	first, _ := repo.Upsert(context.Background(), u)

	u.Name = "Bob Updated"
	u.Email = "bob-new@example.com"
	second, err := repo.Upsert(context.Background(), u)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if second.ID != first.ID {
		t.Error("expected same ID on update")
	}
	if second.Name != "Bob Updated" {
		t.Errorf("name not updated: got %q", second.Name)
	}
}

func TestUserRepository_FindByID_ReturnsUser(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	u := &user.User{
		Email: "carol@example.com", Name: "Carol",
		Role: user.RoleAdmin, Provider: user.ProviderGoogle, ProviderID: "google-003",
	}
	created, _ := repo.Upsert(context.Background(), u)

	found, err := repo.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Email != created.Email {
		t.Errorf("email mismatch: got %q", found.Email)
	}
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, ckerrors.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

// ---- PATRepository ----

func TestPATRepository_SaveAndFindByID(t *testing.T) {
	truncate(t)
	repo := NewPATRepository(testDB)

	saved, err := repo.Save(context.Background(), &pat.PAT{KeyHash: "hash1", Salt: "salt1"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}

	found, err := repo.FindByID(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.KeyHash != "hash1" {
		t.Errorf("KeyHash mismatch: got %q", found.KeyHash)
	}
}

func TestPATRepository_FindByID_NotFound(t *testing.T) {
	truncate(t)
	repo := NewPATRepository(testDB)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, ckerrors.ErrPATNotFound) {
		t.Errorf("expected ErrPATNotFound, got: %v", err)
	}
}

func TestPATRepository_Delete_RemovesPAT(t *testing.T) {
	truncate(t)
	repo := NewPATRepository(testDB)

	saved, _ := repo.Save(context.Background(), &pat.PAT{KeyHash: "h", Salt: "s"})

	if err := repo.Delete(context.Background(), saved.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.FindByID(context.Background(), saved.ID)
	if !errors.Is(err, ckerrors.ErrPATNotFound) {
		t.Errorf("expected ErrPATNotFound after delete, got: %v", err)
	}
}

func TestPATRepository_Delete_NotFound(t *testing.T) {
	truncate(t)
	repo := NewPATRepository(testDB)

	err := repo.Delete(context.Background(), uuid.New())
	if !errors.Is(err, ckerrors.ErrPATNotFound) {
		t.Errorf("expected ErrPATNotFound, got: %v", err)
	}
}

func TestPATRepository_FindAll_ReturnsAll(t *testing.T) {
	truncate(t)
	repo := NewPATRepository(testDB)

	for i := range 3 {
		_, err := repo.Save(context.Background(), &pat.PAT{
			KeyHash: fmt.Sprintf("hash%d", i),
			Salt:    fmt.Sprintf("salt%d", i),
		})
		if err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	all, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 PATs, got %d", len(all))
	}
}

func TestUserRepository_CountAll_ReturnsZeroOnEmptyDB(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	n, err := repo.CountAll(context.Background())
	if err != nil {
		t.Fatalf("CountAll: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestUserRepository_CountAll_ReturnsCorrectCount(t *testing.T) {
	truncate(t)
	repo := NewUserRepository(testDB)

	for i := range 3 {
		_, err := repo.Upsert(context.Background(), &user.User{
			Email: fmt.Sprintf("user%d@example.com", i), Name: fmt.Sprintf("User%d", i),
			Role: user.RoleUser, Provider: user.ProviderGoogle, ProviderID: fmt.Sprintf("g-%d", i),
		})
		if err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}

	n, err := repo.CountAll(context.Background())
	if err != nil {
		t.Fatalf("CountAll: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestPATRepository_Save_WithExpiry(t *testing.T) {
	truncate(t)
	repo := NewPATRepository(testDB)

	exp := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Millisecond)
	saved, err := repo.Save(context.Background(), &pat.PAT{KeyHash: "h", Salt: "s", ExpiresAt: &exp})
	if err != nil {
		t.Fatalf("Save with expiry: %v", err)
	}
	if saved.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set")
	}
}
