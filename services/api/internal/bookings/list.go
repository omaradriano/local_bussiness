package bookings

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// ActiveStatuses still hold a time, a delivery or a unit.
var ActiveStatuses = []string{"pending_deposit", "confirmed", "in_progress", "ready", "out_for_delivery", "checked_out", "overdue"}

type ListFilter struct {
	From, To   time.Time // bookings overlapping [From, To)
	Kind       string    // "" = all
	OnlyActive bool
}

// Summary is one row of a calendar or agenda.
type Summary struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
	CustomerName  string     `json:"customer_name"`
	CustomerPhone string     `json:"customer_phone"`
	Items         string     `json:"items"` // "2 × Rotomartillo"
	Units         []string   `json:"units"` // codes
	Staff         []string   `json:"staff"` // names
	Total         int64      `json:"total"`
	Deposit       int64      `json:"deposit_required"`
}

const maxListRows = 500

// List returns the business's bookings overlapping the window, by start time.
// A booking without end (orders) counts at its start instant.
func List(ctx context.Context, db store.DB, businessID string, f ListFilter) ([]Summary, error) {
	statuses := ActiveStatuses
	if !f.OnlyActive {
		statuses = nil
	}
	rows, err := db.Query(ctx, `
		select b.id::text, b.kind, b.status, b.starts_at, b.ends_at, c.name, c.phone,
		       coalesce((select string_agg(bi.quantity || ' × ' || i.name, ', ' order by i.name)
		                 from public.booking_items bi join public.catalog_items i on i.id = bi.item_id where bi.booking_id = b.id), ''),
		       coalesce((select array_agg(u.code order by u.code) from public.booking_resources r
		                 join public.item_units u on u.id = r.item_unit_id where r.booking_id = b.id), '{}'),
		       coalesce((select array_agg(s.name order by s.name) from public.booking_resources r
		                 join public.staff s on s.id = r.staff_id where r.booking_id = b.id), '{}'),
		       b.total, b.deposit_required
		from public.bookings b join public.customers c on c.id = b.customer_id
		where b.business_id = $1
		  and b.starts_at < $3 and coalesce(b.ends_at, b.starts_at + interval '1 second') > $2
		  and ($4 = '' or b.kind = $4)
		  and ($5::text[] is null or b.status = any($5))
		order by b.starts_at
		limit $6`, businessID, f.From, f.To, f.Kind, statuses, maxListRows)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Summary])
	if out == nil {
		out = []Summary{}
	}
	return out, err
}

// History groups: what the business is doing now, what's finished, and what fell through.
var historyGroups = map[string][]string{
	"active":    ActiveStatuses,
	"finished":  {"returned", "completed"},
	"cancelled": {"cancelled", "expired", "no_show"},
}

type HistoryFilter struct {
	Group      string     // active | finished | cancelled | "" (all)
	Query      string     // customer name or phone digits, or item name
	CustomerID string     // only this customer's bookings
	Before     *time.Time // cursor: bookings starting before this (newest first)
	BeforeID   string
	Limit      int
}

// ValidHistoryGroup reports whether g is a known group ("" = all).
func ValidHistoryGroup(g string) bool { _, ok := historyGroups[g]; return ok || g == "" }

// History lists bookings newest first, for the history screen and a customer's profile.
// Paged by (starts_at, id) so new rows never shift a page.
func History(ctx context.Context, db store.DB, businessID string, f HistoryFilter) ([]Summary, error) {
	limit := min(max(f.Limit, 1), 100)
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, f.Query)
	rows, err := db.Query(ctx, `
		select b.id::text, b.kind, b.status, b.starts_at, b.ends_at, c.name, c.phone,
		       coalesce((select string_agg(bi.quantity || ' × ' || i.name, ', ' order by i.name)
		                 from public.booking_items bi join public.catalog_items i on i.id = bi.item_id where bi.booking_id = b.id), ''),
		       coalesce((select array_agg(u.code order by u.code) from public.booking_resources r
		                 join public.item_units u on u.id = r.item_unit_id where r.booking_id = b.id), '{}'),
		       coalesce((select array_agg(s.name order by s.name) from public.booking_resources r
		                 join public.staff s on s.id = r.staff_id where r.booking_id = b.id), '{}'),
		       b.total, b.deposit_required
		from public.bookings b join public.customers c on c.id = b.customer_id
		where b.business_id = $1
		  and ($2::text[] is null or b.status = any($2))
		  and ($3 = '' or c.id::text = $3)
		  and ($4 = '' or c.name ilike '%' || $4 || '%' or ($5 <> '' and c.phone like '%' || $5 || '%')
		       or exists (select 1 from public.booking_items bi join public.catalog_items i on i.id = bi.item_id
		                  where bi.booking_id = b.id and i.name ilike '%' || $4 || '%'))
		  and ($6::timestamptz is null or (b.starts_at, b.id::text) < ($6, $7))
		order by b.starts_at desc, b.id desc
		limit $8`, businessID, historyGroups[f.Group], f.CustomerID, strings.TrimSpace(f.Query), digits, f.Before, f.BeforeID, limit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Summary])
	if out == nil {
		out = []Summary{}
	}
	return out, err
}
