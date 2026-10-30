package catalog

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrInUse: it has bookings that are active or still to come; finish or cancel them first.
var ErrInUse = errors.New("in use by active bookings")

// finished bookings no longer need the item or the unit.
const activeBooking = `b.status not in ('completed', 'cancelled', 'expired', 'no_show')`

// Delete removes an item from the catalog (and its units): it disappears from lists, inventory and
// availability, but past bookings keep its name. Refused while it has active or upcoming bookings.
func (r Repo) Delete(ctx context.Context, businessID, itemID string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var busy bool
	err = tx.QueryRow(ctx, `select exists (select 1 from public.booking_items bi join public.bookings b on b.id = bi.booking_id
		  where bi.item_id = i.id and `+activeBooking+`)
		from public.catalog_items i where i.business_id = $1 and i.id = $2 and i.deleted_at is null for update`, businessID, itemID).Scan(&busy)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if busy {
		return ErrInUse
	}
	for _, q := range []string{
		`update public.catalog_items set active = false, deleted_at = now() where id = $1`,
		`update public.item_units set status = 'retired', deleted_at = now() where item_id = $1 and deleted_at is null`,
		`delete from public.staff_services where item_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, itemID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DeleteUnit removes one unit; its code can be used again. Refused while it has active or upcoming rentals.
func (r Repo) DeleteUnit(ctx context.Context, businessID, itemID, unitID string) error {
	var busy bool
	err := r.DB.QueryRow(ctx, `select exists (select 1 from public.booking_resources br join public.bookings b on b.id = br.booking_id
		  where br.item_unit_id = u.id and `+activeBooking+`)
		from public.item_units u where u.business_id = $1 and u.item_id = $2 and u.id = $3 and u.deleted_at is null`,
		businessID, itemID, unitID).Scan(&busy)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if busy {
		return ErrInUse
	}
	_, err = r.DB.Exec(ctx, `update public.item_units set status = 'retired', deleted_at = now()
		where id = $1 and deleted_at is null`, unitID)
	return err
}
