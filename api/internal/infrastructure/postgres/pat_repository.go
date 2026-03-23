package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/courtknights/courtknights/internal/domain/ckerrors"
	"github.com/courtknights/courtknights/internal/domain/pat"
)

// PATRepository implements pat.PATRepository using PostgreSQL.
type PATRepository struct {
	db *pgxpool.Pool
}

// NewPATRepository returns a PATRepository backed by the given connection pool.
func NewPATRepository(db *pgxpool.Pool) *PATRepository {
	return &PATRepository{db: db}
}

// FindAll implements pat.PATRepository.
func (r *PATRepository) FindAll(ctx context.Context) ([]*pat.PAT, error) {
	const q = `SELECT id, key_hash, salt, expires_at, created_at FROM personal_access_tokens`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: FindAll PATs: %w", err)
	}
	defer rows.Close()

	var pats []*pat.PAT
	for rows.Next() {
		p, err := scanPAT(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: FindAll PATs scan: %w", err)
		}
		pats = append(pats, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: FindAll PATs rows: %w", err)
	}
	return pats, nil
}

// FindByID implements pat.PATRepository.
func (r *PATRepository) FindByID(ctx context.Context, id uuid.UUID) (*pat.PAT, error) {
	const q = `SELECT id, key_hash, salt, expires_at, created_at FROM personal_access_tokens WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	p, err := scanPAT(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: FindByID PAT: %w", ckerrors.ErrPATNotFound)
		}
		return nil, fmt.Errorf("postgres: FindByID PAT: %w", err)
	}
	return p, nil
}

// Save implements pat.PATRepository.
func (r *PATRepository) Save(ctx context.Context, p *pat.PAT) (*pat.PAT, error) {
	const q = `
		INSERT INTO personal_access_tokens (key_hash, salt, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, key_hash, salt, expires_at, created_at`

	row := r.db.QueryRow(ctx, q, p.KeyHash, p.Salt, p.ExpiresAt)
	result, err := scanPAT(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: Save PAT: %w", err)
	}
	return result, nil
}

// Delete implements pat.PATRepository.
func (r *PATRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM personal_access_tokens WHERE id = $1`

	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: Delete PAT: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: Delete PAT: %w", ckerrors.ErrPATNotFound)
	}
	return nil
}

// scanPAT reads a PAT row from a pgx scanner (pgx.Row or pgx.Rows).
func scanPAT(scanner interface {
	Scan(dest ...any) error
}) (*pat.PAT, error) {
	var p pat.PAT
	var expiresAt *time.Time
	var createdAt time.Time
	err := scanner.Scan(&p.ID, &p.KeyHash, &p.Salt, &expiresAt, &createdAt)
	if err != nil {
		return nil, err
	}
	p.ExpiresAt = expiresAt
	p.CreatedAt = createdAt
	return &p, nil
}
