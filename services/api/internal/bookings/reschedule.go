package bookings

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// reschedulable: an order can change its date until it leaves or is handed over.
var reschedulable = []string{"pending_deposit", "confirmed", "in_progress", "ready"}

// Reschedule moves an order to another date (and optional same-day window), with the same timing
// and capacity rules as a new one. Moving it within its own day doesn't re-check the capacity
// (the order already counts there).
func Reschedule(ctx context.Context, db store.DB, businessID, id string, start time.Time, end *time.Time, actor string, now time.Time) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, id)
	if err != nil {
		return Booking{}, err
	}
	rules := rulesByKind[b.kind]
	if !rules.movable || !slices.Contains(reschedulable, b.status) {
		return Booking{}, Invalid{"status", "En su estado actual ya no cambia de fecha."}
	}
	var old time.Time
	var tz, itemID string
	var qty int
	if err := tx.QueryRow(ctx, `select bk.starts_at, biz.timezone, bi.item_id::text, bi.quantity
		from public.bookings bk join public.businesses biz on biz.id = bk.business_id
		join public.booking_items bi on bi.booking_id = bk.id where bk.id = $1 limit 1`, id).Scan(&old, &tz, &itemID, &qty); err != nil {
		return Booking{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Booking{}, err
	}
	r := &reservation{ctx: ctx, tx: tx, loader: availability.Loader{DB: tx}, businessID: businessID,
		in: &CreateInput{ItemID: itemID, Quantity: qty, StartsAt: &start, EndsAt: end}, loc: loc, now: now, start: start}
	if err := rules.timing(r); err != nil {
		return Booking{}, err
	}
	if day := "2006-01-02"; start.In(loc).Format(day) != old.In(loc).Format(day) {
		if err := rules.resource.check(r); err != nil {
			return Booking{}, err
		}
	}
	if _, err := tx.Exec(ctx, `update public.bookings set starts_at = $2, ends_at = $3, updated_at = now() where id = $1`, id, start, r.end); err != nil {
		return Booking{}, err
	}
	note := fmt.Sprintf("Fecha cambiada: %s → %s", old.In(loc).Format("02/01/2006 15:04"), start.In(loc).Format("02/01/2006 15:04"))
	if _, err := tx.Exec(ctx, `insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, note)
		values ($1, $2, $3, $3, nullif($4, '')::uuid, $5)`, businessID, id, b.status, actor, note); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
}
