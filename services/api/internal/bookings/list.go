package bookings

import (
	"context"
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
