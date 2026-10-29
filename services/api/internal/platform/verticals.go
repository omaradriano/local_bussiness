package platform

import (
	"context"
	"errors"
	"slices"

	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
)

// Who can open a new business of a giro (platform_verticals). Existing businesses keep working
// whatever the status: it only gates new ones.
const (
	Public    = "public" // anyone, at sign-up
	AdminOnly = "admin"  // only the platform admin, to try a giro out (also in production)
	Off       = "off"    // nobody
)

var (
	Statuses           = []string{Public, AdminOnly, Off}
	ErrUnknownVertical = errors.New("unknown vertical")
	ErrBadStatus       = errors.New("unknown status")
)

type Vertical struct {
	Vertical string `json:"vertical"`
	Status   string `json:"status"`
}

func CanCreate(status string, platformAdmin bool) bool {
	return status == Public || (status == AdminOnly && platformAdmin)
}

// AvailableFor: the giros this user can pick when opening a business, in the same order.
func AvailableFor(all []Vertical, platformAdmin bool) []Vertical {
	out := []Vertical{}
	for _, v := range all {
		if CanCreate(v.Status, platformAdmin) {
			out = append(out, v)
		}
	}
	return out
}

// withDefaults: every giro with a template, with its stored status (a new one starts admin-only).
func withDefaults(verticals []string, stored map[string]string) []Vertical {
	out := make([]Vertical, len(verticals))
	for i, v := range verticals {
		s, ok := stored[v]
		if !ok {
			s = AdminOnly
		}
		out[i] = Vertical{v, s}
	}
	return out
}

func (r Repo) Verticals(ctx context.Context) ([]Vertical, error) {
	rows, err := r.DB.Query(ctx, `select vertical, status from public.platform_verticals`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stored := map[string]string{}
	for rows.Next() {
		var v, s string
		if err := rows.Scan(&v, &s); err != nil {
			return nil, err
		}
		stored[v] = s
	}
	return withDefaults(businesses.Verticals, stored), rows.Err()
}

func (r Repo) VerticalStatus(ctx context.Context, vertical string) (string, error) {
	all, err := r.Verticals(ctx)
	if err != nil {
		return "", err
	}
	for _, v := range all {
		if v.Vertical == vertical {
			return v.Status, nil
		}
	}
	return "", ErrUnknownVertical
}

func (r Repo) SetVerticalStatus(ctx context.Context, vertical, status string) (Vertical, error) {
	if !slices.Contains(businesses.Verticals, vertical) {
		return Vertical{}, ErrUnknownVertical
	}
	if !slices.Contains(Statuses, status) {
		return Vertical{}, ErrBadStatus
	}
	_, err := r.DB.Exec(ctx, `insert into public.platform_verticals (vertical, status) values ($1, $2)
		on conflict (vertical) do update set status = excluded.status, updated_at = now()`, vertical, status)
	return Vertical{vertical, status}, err
}
