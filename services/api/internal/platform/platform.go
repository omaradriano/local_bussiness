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
}

type Repo struct{ DB store.DB }

const cols = `id::text, slug, name, vertical, created_at, feature_grants, settings`

func scan(row pgx.Row) (Business, error) {
	var b Business
	var grants, settings []byte
	if err := row.Scan(&b.ID, &b.Slug, &b.Name, &b.Vertical, &b.CreatedAt, &grants, &settings); err != nil {
		return b, err
	}
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
	rows, err := r.DB.Query(ctx, `select `+cols+` from public.businesses order by created_at desc limit 500`)
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
	b, err := scan(r.DB.QueryRow(ctx, `select `+cols+` from public.businesses where id = $1`, id))
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
