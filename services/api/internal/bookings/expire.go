package bookings

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// OverdueGrace: a rental is overdue this long after its return time.
const OverdueGrace = time.Hour

// ExpireHolds moves holds whose advance wasn't paid in time to "expired", releasing what they held.
func ExpireHolds(ctx context.Context, db store.DB, now time.Time) (int, error) {
	return sweep(ctx, db, `select id::text, business_id::text, status, kind from public.bookings
		where status = 'pending_deposit' and hold_expires_at <= $1
		order by hold_expires_at limit 500 for update skip locked`, now, "expired", "Apartado vencido sin anticipo")
}

// MarkOverdue flags rentals that should be back by now (plus grace) and aren't.
func MarkOverdue(ctx context.Context, db store.DB, now time.Time) (int, error) {
	// With «el día de regreso cuenta completo» it's late only once that local day is over.
	return sweep(ctx, db, `select bk.id::text, bk.business_id::text, bk.status, bk.kind from public.bookings bk
		join public.businesses b on b.id = bk.business_id
		where bk.kind = 'rental' and bk.status = 'checked_out'
		  and case when coalesce((b.settings->>'return_by_end_of_day')::boolean, false)
		      then (date_trunc('day', bk.ends_at at time zone b.timezone) + interval '1 day') at time zone b.timezone
		      else bk.ends_at end <= $1
		order by bk.ends_at limit 500 for update of bk skip locked`, now.Add(-OverdueGrace), "overdue", "No regresó a tiempo")
}

// sweep moves every selected booking to `to` in one transaction; rows locked by someone else are skipped.
func sweep(ctx context.Context, db store.DB, query string, at time.Time, to, note string) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, query, at)
	if err != nil {
		return 0, err
	}
	type row struct{ ID, BusinessID, Status, Kind string }
	found, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return 0, err
	}
	for _, r := range found {
		if !CanTransition(r.Kind, r.Status, to) {
			continue
		}
		if err := setStatus(ctx, tx, r.BusinessID, r.ID, r.Status, to, "", note); err != nil {
			return 0, err
		}
	}
	return len(found), tx.Commit(ctx)
}
