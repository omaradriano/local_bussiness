package availability

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrItemNotFound = errors.New("item not found or inactive")

// Statuses that no longer hold anything (their resources are released).
const releasedStatuses = `('cancelled', 'expired', 'no_show')`

// Item is the catalog data availability needs.
type Item struct {
	ID              string
	Kind            string
	DurationMinutes int
	BufferMinutes   int
	PrepHours       int
	DailyCapacity   int
}

// Business holds the time zone and the scheduling settings.
type Business struct {
	Loc               *time.Location
	SlotMinutes       int // service slot granularity (settings.slot_minutes, default 30)
	DailyCapacity     int // settings.daily_capacity, 0 = no limit
	InspectionMinutes int // settings.inspection_minutes for rentals
	BufferMinutes     int // settings.buffer_minutes: gap after each appointment (whole business)
}

type Loader struct{ DB store.DB }

func (l Loader) Business(ctx context.Context, businessID string) (Business, error) {
	var tz string
	var raw []byte
	if err := l.DB.QueryRow(ctx, `select timezone, settings from public.businesses where id = $1`, businessID).Scan(&tz, &raw); err != nil {
		return Business{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Business{}, err
	}
	var s struct {
		SlotMinutes       int `json:"slot_minutes"`
		DailyCapacity     int `json:"daily_capacity"`
		InspectionMinutes int `json:"inspection_minutes"`
		BufferMinutes     int `json:"buffer_minutes"`
	}
	_ = json.Unmarshal(raw, &s) // missing keys keep their defaults
	if s.SlotMinutes <= 0 {
		s.SlotMinutes = 30
	}
	return Business{Loc: loc, SlotMinutes: s.SlotMinutes, DailyCapacity: s.DailyCapacity, InspectionMinutes: s.InspectionMinutes,
		BufferMinutes: s.BufferMinutes}, nil
}

func (l Loader) Item(ctx context.Context, businessID, itemID string) (Item, error) {
	var it Item
	err := l.DB.QueryRow(ctx, `
		select id::text, kind, coalesce(duration_minutes, 0), buffer_minutes, coalesce(prep_hours, 0), coalesce(daily_capacity, 0)
		from public.catalog_items where business_id = $1 and id = $2 and active`, businessID, itemID).
		Scan(&it.ID, &it.Kind, &it.DurationMinutes, &it.BufferMinutes, &it.PrepHours, &it.DailyCapacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, ErrItemNotFound
	}
	return it, err
}

// BusinessHours returns the general shifts (staff_id null).
func (l Loader) BusinessHours(ctx context.Context, businessID string) ([]Shift, error) {
	return l.shifts(ctx, `select weekday, to_char(opens_at, 'HH24:MI'), to_char(closes_at, 'HH24:MI')
		from public.business_hours where business_id = $1 and staff_id is null`, businessID)
}

func (l Loader) shifts(ctx context.Context, sql string, args ...any) ([]Shift, error) {
	rows, err := l.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Shift])
}

// Blackouts overlapping the window (business-wide, per person and per unit).
func (l Loader) Blackouts(ctx context.Context, businessID string, w Interval) ([]Blackout, error) {
	rows, err := l.DB.Query(ctx, `
		select coalesce(staff_id::text, ''), coalesce(item_unit_id::text, ''), starts_at, ends_at
		from public.blackouts where business_id = $1 and tstzrange(starts_at, ends_at) && tstzrange($2, $3)`,
		businessID, w.Start, w.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Blackout
	for rows.Next() {
		var b Blackout
		if err := rows.Scan(&b.StaffID, &b.UnitID, &b.Start, &b.End); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ServiceStaff: active people who do the service, with their own shifts and what they have booked in w.
func (l Loader) ServiceStaff(ctx context.Context, businessID, itemID string, w Interval) ([]Staff, error) {
	rows, err := l.DB.Query(ctx, `
		select s.id::text
		from public.staff s join public.staff_services ss on ss.staff_id = s.id
		where s.business_id = $1 and ss.item_id = $2 and s.active
		order by s.created_at`, businessID, itemID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make([]Staff, 0, len(ids))
	for _, id := range ids {
		hours, err := l.shifts(ctx, `select weekday, to_char(opens_at, 'HH24:MI'), to_char(closes_at, 'HH24:MI')
			from public.business_hours where business_id = $1 and staff_id = $2`, businessID, id)
		if err != nil {
			return nil, err
		}
		busy, err := l.busy(ctx, `r.staff_id = $2`, businessID, id, w)
		if err != nil {
			return nil, err
		}
		out = append(out, Staff{ID: id, Hours: hours, Busy: busy})
	}
	return out, nil
}

// RentalUnits: units of the item in status "available", with their rentals overlapping w.
func (l Loader) RentalUnits(ctx context.Context, businessID, itemID string, w Interval) ([]Unit, error) {
	rows, err := l.DB.Query(ctx, `select id::text from public.item_units
		where business_id = $1 and item_id = $2 and status = 'available' order by code`, businessID, itemID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make([]Unit, 0, len(ids))
	for _, id := range ids {
		busy, err := l.busy(ctx, `r.item_unit_id = $2`, businessID, id, w)
		if err != nil {
			return nil, err
		}
		out = append(out, Unit{ID: id, Busy: busy})
	}
	return out, nil
}

func (l Loader) busy(ctx context.Context, who, businessID, id string, w Interval) ([]Interval, error) {
	rows, err := l.DB.Query(ctx, `
		select lower(r.during), upper(r.during)
		from public.booking_resources r join public.bookings b on b.id = r.booking_id
		where r.business_id = $1 and `+who+` and r.during && tstzrange($3, $4)
		  and b.status not in `+releasedStatuses, businessID, id, w.Start, w.End)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Interval])
}

// ProductBooked counts, per local date in [from, to], product orders of the business and units of this item.
func (l Loader) ProductBooked(ctx context.Context, businessID, itemID, tz, from, to string) (map[string]Booked, error) {
	rows, err := l.DB.Query(ctx, `
		select to_char((b.starts_at at time zone $3)::date, 'YYYY-MM-DD') as day,
		       count(distinct b.id),
		       coalesce(sum(bi.quantity) filter (where bi.item_id = $2), 0)
		from public.bookings b
		left join public.booking_items bi on bi.booking_id = b.id
		where b.business_id = $1 and b.kind = 'product' and b.status not in `+releasedStatuses+`
		  and (b.starts_at at time zone $3)::date between $4::date and $5::date
		group by 1`, businessID, itemID, tz, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Booked{}
	for rows.Next() {
		var day string
		var b Booked
		if err := rows.Scan(&day, &b.All, &b.Item); err != nil {
			return nil, err
		}
		out[day] = b
	}
	return out, rows.Err()
}
