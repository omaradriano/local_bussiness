package bookings

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// InventoryRow: one rental unit in one situation. A unit can appear several times
// (rented now and booked again later); a free unit appears once as "available".
type InventoryRow struct {
	State        string     `json:"state"` // available | rented | scheduled | inactive
	Date         *time.Time `json:"date"`  // rented: return due; scheduled: pick-up; otherwise null
	UnitID       string     `json:"unit_id"`
	UnitCode     string     `json:"unit_code"`
	UnitStatus   string     `json:"unit_status"` // available | maintenance | retired
	ItemID       string     `json:"item_id"`
	ItemName     string     `json:"item_name"`
	BookingID    *string    `json:"booking_id"`
	Status       *string    `json:"booking_status"`
	CustomerName *string    `json:"customer_name"`
	StartsAt     *time.Time `json:"starts_at"`
	EndsAt       *time.Time `json:"ends_at"`
}

// rentalState: checked out (or late) = rented; still to go out = scheduled.
func rentalState(bookingStatus string) string {
	if bookingStatus == "checked_out" || bookingStatus == "overdue" {
		return "rented"
	}
	return "scheduled"
}

// SortInventory: dated rows first (soonest first), then free units, then inactive ones; ties by item and code.
func SortInventory(rows []InventoryRow) {
	rank := map[string]int{"rented": 0, "scheduled": 0, "available": 1, "inactive": 2}
	slices.SortStableFunc(rows, func(a, b InventoryRow) int {
		if r := rank[a.State] - rank[b.State]; r != 0 {
			return r
		}
		if a.Date != nil && b.Date != nil && !a.Date.Equal(*b.Date) {
			return a.Date.Compare(*b.Date)
		}
		if a.ItemName != b.ItemName {
			if a.ItemName < b.ItemName {
				return -1
			}
			return 1
		}
		if a.UnitCode < b.UnitCode {
			return -1
		} else if a.UnitCode > b.UnitCode {
			return 1
		}
		return 0
	})
}

// Inventory lists every rental unit of the business with its rentals that are out or still to come.
func Inventory(ctx context.Context, db store.DB, businessID string) ([]InventoryRow, error) {
	rows, err := db.Query(ctx, `
		select u.id::text, u.code, u.status, ci.id::text, ci.name, b.id::text, b.status, b.customer_name, b.starts_at, b.ends_at
		from public.item_units u
		join public.catalog_items ci on ci.id = u.item_id
		left join (
		  select br.item_unit_id, bk.id, bk.status, c.name as customer_name, bk.starts_at, bk.ends_at
		  from public.booking_resources br
		  join public.bookings bk on bk.id = br.booking_id and bk.status in ('pending_deposit', 'confirmed', 'checked_out', 'overdue')
		  join public.customers c on c.id = bk.customer_id
		  where br.business_id = $1
		) b on b.item_unit_id = u.id
		where u.business_id = $1 and ci.kind = 'rental'`, businessID)
	if err != nil {
		return nil, err
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (InventoryRow, error) {
		var x InventoryRow
		err := r.Scan(&x.UnitID, &x.UnitCode, &x.UnitStatus, &x.ItemID, &x.ItemName, &x.BookingID, &x.Status, &x.CustomerName, &x.StartsAt, &x.EndsAt)
		return x, err
	})
	if err != nil {
		return nil, err
	}
	out := []InventoryRow{}
	seen := map[string]bool{} // units already listed as available/inactive
	rentedNow := map[string]bool{}
	for _, x := range all {
		if x.Status != nil && rentalState(*x.Status) == "rented" {
			rentedNow[x.UnitID] = true
		}
	}
	for _, x := range all {
		if x.BookingID != nil {
			x.State = rentalState(*x.Status)
			if x.State == "rented" {
				x.Date = x.EndsAt
			} else {
				x.Date = x.StartsAt
			}
			out = append(out, x)
		}
		if seen[x.UnitID] {
			continue
		}
		seen[x.UnitID] = true
		base := InventoryRow{UnitID: x.UnitID, UnitCode: x.UnitCode, UnitStatus: x.UnitStatus, ItemID: x.ItemID, ItemName: x.ItemName}
		switch {
		case x.UnitStatus != "available":
			base.State = "inactive"
		case rentedNow[x.UnitID]:
			continue // it's out: its "rented" row says so
		default:
			base.State = "available"
		}
		out = append(out, base)
	}
	SortInventory(out)
	return out, nil
}
