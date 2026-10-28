// Package store is the API's Postgres access. The API connects as a privileged role
// (not subject to RLS), so every query must filter by business explicitly.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is satisfied by *pgxpool.Pool and pgx.Tx (tests run inside a rolled-back transaction).
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error) // on a pgx.Tx this opens a savepoint
}

type Store struct{ db DB }

func New(db DB) *Store { return &Store{db: db} }

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func (s *Store) Role(ctx context.Context, businessID, userID string) (string, error) {
	var role string
	err := s.db.QueryRow(ctx,
		`select role from public.business_members where business_id = $1 and user_id = $2`,
		businessID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

type Membership struct {
	BusinessID string   `json:"business_id"`
	Slug       string   `json:"slug"`
	Name       string   `json:"name"`
	Vertical   string   `json:"vertical"`
	Role       string   `json:"role"`
	Features   []string `json:"features"` // active features; filled by the caller from the two below

	Grants   []byte `json:"-"` // businesses.feature_grants
	Settings []byte `json:"-"` // businesses.settings (owner's feature choices)
}

func (s *Store) Memberships(ctx context.Context, userID string) ([]Membership, error) {
	rows, err := s.db.Query(ctx, `
		select b.id::text, b.slug, b.name, b.vertical, m.role, '{}'::text[], b.feature_grants, b.settings
		from public.business_members m
		join public.businesses b on b.id = m.business_id
		where m.user_id = $1
		order by b.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Membership])
}

// IsPlatformAdmin reports whether the user administers the platform (not a business).
func (s *Store) IsPlatformAdmin(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, `select exists (select 1 from public.platform_admins where user_id = $1)`, userID).Scan(&ok)
	return ok, err
}
