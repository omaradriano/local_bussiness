// Package platform is the platform admin's side: every business with its giro and features, and
// the grants that give or remove a feature for one business.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var (
	ErrNotFound       = errors.New("business not found")
	ErrNotForBusiness = errors.New("feature not supported by the business's engine")
)

type Business struct {
	ID        string           `json:"business_id"`
	Slug      string           `json:"slug"`
	Name      string           `json:"name"`
	Vertical  string           `json:"vertical"`
	CreatedAt time.Time        `json:"created_at"`
	Features  []features.State `json:"features"`
	Account   plans.View       `json:"account"` // plan of the owner's account (covers all their businesses)
}

type Repo struct{ DB store.DB }

// The owner's account comes along; an owner without one predates plans (Pro, like plans.Repo).
const cols = `b.id::text, b.slug, b.name, b.vertical, b.created_at, b.feature_grants, b.settings, ap.plan, ap.ends_at`
const from = ` from public.businesses b
	left join lateral (select user_id from public.business_members where business_id = b.id and role = 'owner' order by created_at limit 1) o on true
	left join public.account_plans ap on ap.user_id = o.user_id`

func scan(row pgx.Row) (Business, error) {
	var b Business
	var grants, settings []byte
	var plan *string
	var acct plans.Account
	if err := row.Scan(&b.ID, &b.Slug, &b.Name, &b.Vertical, &b.CreatedAt, &grants, &settings, &plan, &acct.EndsAt); err != nil {
		return b, err
	}
	acct.Plan = plans.Pro
	if plan != nil {
		acct.Plan = *plan
	}
	b.Account = acct.View(time.Now()) // display only; the API checks writes with its own clock
	var s struct {
		Features map[string]bool `json:"features"`
	}
	_ = json.Unmarshal(settings, &s)
	b.Features = features.States(b.Vertical, features.Parse(grants), s.Features)
	if b.Features == nil {
		b.Features = []features.State{}
	}
	return b, nil
}

// Businesses lists every business, newest first.
func (r Repo) Businesses(ctx context.Context) ([]Business, error) {
	rows, err := r.DB.Query(ctx, `select `+cols+from+` order by b.created_at desc limit 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Business{}
	for rows.Next() {
		b, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r Repo) Business(ctx context.Context, id string) (Business, error) {
	b, err := scan(r.DB.QueryRow(ctx, `select `+cols+from+` where b.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// SetGrant gives (true) or removes (false) a feature for a business; nil goes back to what its giro says.
func (r Repo) SetGrant(ctx context.Context, id, feature string, grant *bool) (Business, error) {
	b, err := r.Business(ctx, id)
	if err != nil {
		return b, err
	}
	if !features.Supports(b.Vertical, feature) {
		return b, ErrNotForBusiness
	}
	if grant == nil {
		_, err = r.DB.Exec(ctx, `update public.businesses set feature_grants = feature_grants - $2 where id = $1`, id, feature)
	} else {
		_, err = r.DB.Exec(ctx, `update public.businesses set feature_grants = feature_grants || jsonb_build_object($2::text, $3::boolean) where id = $1`, id, feature, *grant)
	}
	if err != nil {
		return b, err
	}
	return r.Business(ctx, id)
}
