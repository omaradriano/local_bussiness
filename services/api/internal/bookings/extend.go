package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// Extendable: a rental can get a later return date before it comes back.
var extendable = []string{"pending_deposit", "confirmed", "checked_out", "overdue"}

// Extend moves a rental's return date later, keeping the same units (409 if one is taken by then),
// and recalculates the total with the same price and closed-day rules as a new rental.
// An overdue rental extended past now goes back to checked_out.
func Extend(ctx context.Context, db store.DB, businessID, id string, newEnd time.Time, actor string, now time.Time) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, id)
	if err != nil {
		return Booking{}, err
	}
	if !hasCustody(b.kind) || !slices.Contains(extendable, b.status) {
		return Booking{}, Invalid{"status", fmt.Sprintf("Una renta «%s» no se puede extender.", label(b.status))}
	}
	var start, end time.Time
	var tz, priceUnit string
	var rawSettings []byte
	var unitPrice, qty int64
	var opts []byte
	var disc Discount
	if err := tx.QueryRow(ctx, `select bk.starts_at, bk.ends_at, biz.timezone, biz.settings, coalesce(ci.price_unit, 'day'), bi.unit_price, bi.quantity, bi.options,
		  coalesce(bk.discount_type, ''), bk.discount_value
		from public.bookings bk join public.businesses biz on biz.id = bk.business_id
		join public.booking_items bi on bi.booking_id = bk.id join public.catalog_items ci on ci.id = bi.item_id
		where bk.id = $1`, id).Scan(&start, &end, &tz, &rawSettings, &priceUnit, &unitPrice, &qty, &opts, &disc.Type, &disc.Value); err != nil {
		return Booking{}, err
	}
	if !newEnd.After(end) {
		return Booking{}, Invalid{"ends_at", "La nueva fecha de regreso debe ser después de la actual."}
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Booking{}, err
	}
	hours := func() ([]availability.Shift, error) {
		return availability.Loader{DB: tx}.BusinessHours(ctx, businessID)
	}
	periods, err := rulesByKind[b.kind].pricing(periodQuery{loc: loc, hours: hours, rentals: settings.RentalsFrom(rawSettings),
		priceUnit: priceUnit, start: start, end: &newEnd})
	if err != nil {
		return Booking{}, err
	}

	// Same units, longer range; the exclusion constraint refuses it if another rental has one by then.
	if _, err := tx.Exec(ctx, `update public.booking_resources set during = tstzrange(lower(during), $2) where booking_id = $1`, id, newEnd); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return Booking{}, ErrSlotTaken
		}
		return Booking{}, err
	}

	var o map[string]any
	if json.Unmarshal(opts, &o) != nil || o == nil {
		o = map[string]any{}
	}
	extra, _ := o["options_extra"].(float64)
	// A percent discount follows the new total; a fixed one stays.
	q := Compute(QuoteInput{UnitPrice: unitPrice, OptionsExtra: int64(extra), Quantity: qty, Periods: periods, Discount: disc})
	o["periods"] = periods
	opts, _ = json.Marshal(o)
	if _, err := tx.Exec(ctx, `update public.booking_items set options = $2 where booking_id = $1`, id, opts); err != nil {
		return Booking{}, err
	}
	// The advance keeps its proportion of the new total.
	if _, err := tx.Exec(ctx, `update public.bookings set ends_at = $2, subtotal = $3, discount = $4, total = $5, deposit_required = $6, updated_at = now() where id = $1`,
		id, newEnd, q.Subtotal, q.Discount, q.Total, rescaleAdvance(b.depositRequired, b.total, q.Total)); err != nil {
		return Booking{}, err
	}
	note := fmt.Sprintf("Renta extendida hasta %s; nuevo total %s", newEnd.In(loc).Format("02/01/2006 15:04"), mxn(q.Total))
	if b.status == "overdue" && newEnd.After(now) {
		if err := setStatus(ctx, tx, businessID, id, b.status, "checked_out", actor, note); err != nil {
			return Booking{}, err
		}
	} else if _, err := tx.Exec(ctx, `insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, note)
		values ($1, $2, $3, $3, nullif($4, '')::uuid, $5)`, businessID, id, b.status, actor, note); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
}
