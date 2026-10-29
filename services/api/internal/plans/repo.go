package plans

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNoOwner = errors.New("business has no owner")

// DB: what the repo needs (store.DB satisfies it; plans can't import store, which uses its types).
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct{ DB DB }

// ForUser: the account of an owner; ok is false when they never opened a business.
func (r Repo) ForUser(ctx context.Context, userID string) (Account, bool, error) {
	var a Account
	err := r.DB.QueryRow(ctx, `select plan, ends_at from public.account_plans where user_id = $1`, userID).Scan(&a.Plan, &a.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, false, nil
	}
	return a, err == nil, err
}

// ForBusiness: the account of the business's owner (every business has one since its creation).
func (r Repo) ForBusiness(ctx context.Context, businessID string) (Account, error) {
	owner, err := r.OwnerOf(ctx, businessID)
	if err != nil {
		return Account{}, err
	}
	a, ok, err := r.ForUser(ctx, owner)
	if err == nil && !ok {
		a = Account{Plan: Pro} // shortcut: an owner without a row predates plans; the migration gave them all Pro
	}
	return a, err
}

func (r Repo) OwnerOf(ctx context.Context, businessID string) (string, error) {
	var owner string
	err := r.DB.QueryRow(ctx, `select user_id::text from public.business_members where business_id = $1 and role = 'owner'
		order by created_at limit 1`, businessID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoOwner
	}
	return owner, err
}

// StartTrial gives a new owner their trial; an owner who already has an account keeps it.
func (r Repo) StartTrial(ctx context.Context, userID string, now time.Time) error {
	t := NewTrial(now)
	_, err := r.DB.Exec(ctx, `insert into public.account_plans (user_id, plan, ends_at) values ($1, $2, $3) on conflict do nothing`,
		userID, t.Plan, t.EndsAt)
	return err
}

// Set: the platform admin assigns a plan (while there's no billing).
func (r Repo) Set(ctx context.Context, userID string, a Account, note string) error {
	_, err := r.DB.Exec(ctx, `insert into public.account_plans (user_id, plan, ends_at, note) values ($1, $2, $3, nullif($4, ''))
		on conflict (user_id) do update set plan = excluded.plan, ends_at = excluded.ends_at, note = excluded.note, updated_at = now()`,
		userID, a.Plan, a.EndsAt, note)
	return err
}
