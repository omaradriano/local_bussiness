// Package exports reads everything of a business, unpaginated, for the owner's CSV exports. The app
// turns the rows into CSV with the giro's own words (packages/shared); here they stay raw (centavos,
// UTC instants, status keys), with the business's time zone to show them in.
package exports

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/bookings"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// Kinds that can be exported.
var Kinds = []string{"customers", "bookings", "expenses", "sales", "catalog", "products"}

func Valid(kind string) bool { return slices.Contains(Kinds, kind) }

// Period filters dated kinds (bookings by start, expenses, sales); nil = everything.
type Period struct{ From, To time.Time } // business dates, both included

const maxRows = 50_000

type Customer struct {
	Name              string     `json:"name"`
	Phone             string     `json:"phone"`
	Email             *string    `json:"email"`
	Address           *string    `json:"address"`
	GuarantorName     *string    `json:"guarantor_name"`
	GuarantorPhone    *string    `json:"guarantor_phone"`
	Trusted           bool       `json:"trusted"`
	Notes             *string    `json:"notes"`
	PrivacyAcceptedAt *time.Time `json:"privacy_accepted_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

type Booking struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Status    string     `json:"status"`
	Source    string     `json:"source"`
	StartsAt  *time.Time `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	Customer  string     `json:"customer"`
	Phone     string     `json:"phone"`
	Items     string     `json:"items"`
	Total     int64      `json:"total"`
	Paid      int64      `json:"paid"`
	Due       int64      `json:"due"`
	Extra     int64      `json:"extra_charges"`
	CreatedAt time.Time  `json:"created_at"`
}

type Expense struct {
	SpentOn     time.Time `json:"spent_on"`
	Category    string    `json:"category"`
	Description *string   `json:"description"`
	Amount      int64     `json:"amount"`
	Recurring   bool      `json:"recurring"` // a confirmed charge of a recurring expense
}

type Sale struct {
	SoldAt   time.Time `json:"sold_at"`
	Method   string    `json:"method"`
	Products string    `json:"products"`
	Total    int64     `json:"total"`
	Cost     int64     `json:"cost"`
}

type CatalogItem struct {
	Kind            string  `json:"kind"`
	Name            string  `json:"name"`
	Price           int64   `json:"price"`
	PriceUnit       *string `json:"price_unit"`
	DurationMinutes *int    `json:"duration_minutes"`
	SecurityDeposit *int64  `json:"security_deposit"`
	LateFeePerDay   *int64  `json:"late_fee_per_day"`
	Active          bool    `json:"active"`
}

type Product struct {
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Cost   int64  `json:"cost"`
	Stock  int    `json:"stock"`
	Active bool   `json:"active"`
}

type Repo struct{ DB store.DB }

// Timezone of the business (what the app formats dates in).
func (r Repo) Timezone(ctx context.Context, businessID string) (string, error) {
	var tz string
	err := r.DB.QueryRow(ctx, `select timezone from public.businesses where id = $1`, businessID).Scan(&tz)
	return tz, err
}

// Rows of one kind (Valid(kind) must hold).
func (r Repo) Rows(ctx context.Context, businessID, kind string, p *Period) (any, error) {
	tz, err := r.Timezone(ctx, businessID)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	// Dated kinds: [from 00:00, to + 1 day) in the business's time zone; no period = all of it.
	fromDay, toDay := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9998, 12, 31, 0, 0, 0, 0, time.UTC)
	if p != nil {
		fromDay, toDay = p.From, p.To
	}
	start := time.Date(fromDay.Year(), fromDay.Month(), fromDay.Day(), 0, 0, 0, 0, loc)
	end := time.Date(toDay.Year(), toDay.Month(), toDay.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	switch kind {
	case "customers":
		return collect[Customer](ctx, r.DB, `select name, phone, email, address, guarantor_name, guarantor_phone, trusted, notes,
			privacy_accepted_at, created_at from public.customers where business_id = $1 and not walk_in order by name limit $2`, businessID, maxRows)
	case "bookings":
		return r.bookings(ctx, businessID, start, end)
	case "expenses":
		return collect[Expense](ctx, r.DB, `select spent_on::timestamp, category, description, amount, recurring_id is not null
			from public.expenses where business_id = $1 and spent_on between $2 and $3 order by spent_on, created_at limit $4`,
			businessID, fromDay, toDay, maxRows)
	case "sales":
		return collect[Sale](ctx, r.DB, `select s.sold_at, s.method,
			coalesce((select string_agg(l.quantity || ' × ' || p.name, ', ' order by p.name) from public.sale_lines l
			          join public.retail_products p on p.id = l.product_id where l.sale_id = s.id), ''), s.total, s.cost_total
			from public.sales s where s.business_id = $1 and s.sold_at >= $2 and s.sold_at < $3 order by s.sold_at limit $4`, businessID, start, end, maxRows)
	case "catalog":
		return collect[CatalogItem](ctx, r.DB, `select kind, name, price, price_unit, duration_minutes, security_deposit, late_fee_per_day, active
			from public.catalog_items where business_id = $1 order by kind, sort, name limit $2`, businessID, maxRows)
	case "products":
		return collect[Product](ctx, r.DB, `select name, price, cost, stock, active from public.retail_products
			where business_id = $1 order by name limit $2`, businessID, maxRows)
	}
	return nil, nil
}

// bookings: paid and due with the same rule as the booking's detail (bookings.ComputeBalance).
func (r Repo) bookings(ctx context.Context, businessID string, start, end time.Time) ([]Booking, error) {
	rows, err := r.DB.Query(ctx, `
		select b.id::text, b.kind, b.status, b.source, b.starts_at, b.ends_at, c.name, c.phone,
		       coalesce((select string_agg(i.name || case when bi.quantity > 1 then ' × ' || bi.quantity else '' end, ', ' order by i.name)
		                 from public.booking_items bi join public.catalog_items i on i.id = bi.item_id where bi.booking_id = b.id), ''),
		       b.total, b.created_at,
		       coalesce((select jsonb_agg(jsonb_build_object('type', p.type, 'amount', p.amount)) from public.payments p where p.booking_id = b.id), '[]')
		from public.bookings b join public.customers c on c.id = b.customer_id
		where b.business_id = $1 and coalesce(b.starts_at, b.created_at) >= $2 and coalesce(b.starts_at, b.created_at) < $3
		order by coalesce(b.starts_at, b.created_at) limit $4`, businessID, start, end, maxRows)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Booking{}
	for rows.Next() {
		var b Booking
		var pays []bookings.Payment
		if err := rows.Scan(&b.ID, &b.Kind, &b.Status, &b.Source, &b.StartsAt, &b.EndsAt, &b.Customer, &b.Phone, &b.Items, &b.Total, &b.CreatedAt, &pays); err != nil {
			return nil, err
		}
		bal := bookings.ComputeBalance(b.Total, pays)
		b.Paid, b.Due, b.Extra = bal.Paid, bal.Due, bal.Extra
		out = append(out, b)
	}
	return out, rows.Err()
}

func collect[T any](ctx context.Context, db store.DB, q string, args ...any) ([]T, error) {
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[T])
	if out == nil {
		out = []T{}
	}
	return out, err
}
