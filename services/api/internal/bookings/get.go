package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
	"github.com/omaradriano/local_bussiness/services/api/internal/steps"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("booking not found")

type Customer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	WalkIn bool   `json:"walk_in"` // the business's «Cliente casual» (no profile, no WhatsApp)
}

type Line struct {
	ItemID    string         `json:"item_id"`
	Name      string         `json:"name"`
	Quantity  int            `json:"quantity"`
	UnitPrice int64          `json:"unit_price"`
	Options   map[string]any `json:"options"`
}

type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Booking struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          *time.Time `json:"ends_at"`
	Customer        Customer   `json:"customer"`
	Items           []Line     `json:"items"`
	Staff           []Ref      `json:"staff"` // who attends (service)
	Units           []Ref      `json:"units"` // which pieces go out (rental); Name = code
	Fulfillment     *string    `json:"fulfillment"`
	DeliveryAddress *string    `json:"delivery_address"`
	Subtotal        int64      `json:"subtotal"`
	DepositRequired int64      `json:"deposit_required"`
	Total           int64      `json:"total"`
	Notes           *string    `json:"notes"`
	Source          string     `json:"source"`
	HoldExpiresAt   *time.Time `json:"hold_expires_at"`
	PublicToken     string     `json:"public_token"`
	CreatedAt       time.Time  `json:"created_at"`

	Payments  []Payment   `json:"payments"`
	Documents []Document  `json:"documents"` // files attached with a free name
	Balance   Balance     `json:"balance"`
	Events    []Event     `json:"events"`
	Steps     []Step      `json:"steps"`
	Next      []string    `json:"next"` // status moves available from the panel
	Rental    *RentalInfo `json:"rental"`
}

// RentalInfo: when the units went out and came back, and the late fee it suggests.
type RentalInfo struct {
	CheckedOutAt     *time.Time `json:"checked_out_at"`
	ReturnedAt       *time.Time `json:"returned_at"`
	OutCondition     *string    `json:"out_condition"`
	ReturnCondition  *string    `json:"return_condition"`
	LateDays         int64      `json:"late_days"`
	LateFeeSuggested int64      `json:"late_fee_suggested"`
}

type Event struct {
	FromStatus *string   `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	At         time.Time `json:"at"`
	Note       *string   `json:"note"`
}

// Step is a copied workflow step (e.g. "Foto del INE" before checked_out) and its result.
type Step struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	StepType    string          `json:"step_type"`
	ToStatus    *string         `json:"to_status"`
	Required    bool            `json:"required"`
	Status      string          `json:"status"` // pending | done | skipped
	CompletedAt *time.Time      `json:"completed_at"`
	Files       []string        `json:"files"` // S3 keys (private buckets)
	Config      json.RawMessage `json:"config"`
	Bucket      string          `json:"bucket"`       // where its files live ("" = no files)
	CustomerDoc string          `json:"customer_doc"` // "" or the customer document it takes (ine, proof_of_address)
	OnFile      bool            `json:"on_file"`      // that document is already in the customer's profile (or they're trusted)
}

// Get returns one booking of the business with its customer, lines and assigned staff/units.
func Get(ctx context.Context, db store.DB, businessID, id string) (Booking, error) {
	var b Booking
	var items, staff, units []byte
	err := db.QueryRow(ctx, `
		select b.id::text, b.kind, b.status, b.starts_at, b.ends_at,
		       c.id::text, c.name, c.phone, c.walk_in,
		       b.fulfillment, b.delivery_address, b.subtotal, b.deposit_required, b.total, b.notes, b.source,
		       b.hold_expires_at, b.public_token, b.created_at,
		       coalesce((select jsonb_agg(jsonb_build_object('item_id', bi.item_id, 'name', i.name, 'quantity', bi.quantity,
		                 'unit_price', bi.unit_price, 'options', bi.options) order by i.name)
		                 from public.booking_items bi join public.catalog_items i on i.id = bi.item_id where bi.booking_id = b.id), '[]'),
		       coalesce((select jsonb_agg(jsonb_build_object('id', s.id, 'name', s.name) order by s.name)
		                 from public.booking_resources r join public.staff s on s.id = r.staff_id where r.booking_id = b.id), '[]'),
		       coalesce((select jsonb_agg(jsonb_build_object('id', u.id, 'name', u.code) order by u.code)
		                 from public.booking_resources r join public.item_units u on u.id = r.item_unit_id where r.booking_id = b.id), '[]')
		from public.bookings b join public.customers c on c.id = b.customer_id
		where b.business_id = $1 and b.id = $2`, businessID, id).Scan(
		&b.ID, &b.Kind, &b.Status, &b.StartsAt, &b.EndsAt, &b.Customer.ID, &b.Customer.Name, &b.Customer.Phone, &b.Customer.WalkIn,
		&b.Fulfillment, &b.DeliveryAddress, &b.Subtotal, &b.DepositRequired, &b.Total, &b.Notes, &b.Source,
		&b.HoldExpiresAt, &b.PublicToken, &b.CreatedAt, &items, &staff, &units)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	for _, x := range []struct {
		raw []byte
		dst any
	}{{items, &b.Items}, {staff, &b.Staff}, {units, &b.Units}} {
		if err := json.Unmarshal(x.raw, x.dst); err != nil {
			return b, err
		}
	}

	if b.Payments, err = payments(ctx, db, b.ID); err != nil {
		return b, err
	}
	if b.Documents, err = documents(ctx, db, b.ID); err != nil {
		return b, err
	}
	b.Balance = ComputeBalance(b.Total, b.Payments)
	b.Next = Next(b.Kind, b.Status)
	if b.Next == nil {
		b.Next = []string{}
	}

	rows, err := db.Query(ctx, `select from_status, to_status, at, note from public.booking_events where booking_id = $1 order by at, id`, b.ID)
	if err != nil {
		return b, err
	}
	if b.Events, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Event]); err != nil {
		return b, err
	}

	rows, err = db.Query(ctx, `select bs.id::text, coalesce(bs.step_snapshot->>'label', ''), coalesce(bs.step_snapshot->>'step_type', ''),
		bs.step_snapshot->>'to_status', coalesce((bs.step_snapshot->>'required')::boolean, true), bs.status, bs.completed_at, bs.s3_keys,
		coalesce(bs.step_snapshot->'config', '{}'), '', coalesce(bs.step_snapshot->'config'->>'customer_doc', ''),
		bs.step_snapshot->'config' ? 'customer_doc' and (c.trusted or exists (select 1 from public.customer_documents cd
		  where cd.customer_id = c.id and cd.doc_type = bs.step_snapshot->'config'->>'customer_doc'))
		from public.booking_steps bs join public.bookings bk on bk.id = bs.booking_id join public.customers c on c.id = bk.customer_id
		where bs.booking_id = $1 order by (bs.step_snapshot->>'sort')::int nulls last, bs.id`, b.ID)
	if err != nil {
		return b, err
	}
	if b.Steps, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Step]); err != nil {
		return b, err
	}
	if b.Steps == nil {
		b.Steps = []Step{}
	}
	for i := range b.Steps {
		if st, ok := steps.Lookup(b.Steps[i].StepType); ok {
			b.Steps[i].Bucket = st.Bucket(b.Steps[i].Config)
		}
	}
	if hasCustody(b.Kind) {
		var r RentalInfo
		var dailyPrice int64
		var qty int
		var rawSettings []byte
		var tz string
		err := db.QueryRow(ctx, `
			select min(rc.out_at), max(rc.returned_at), min(rc.out_condition), min(rc.return_condition),
			       coalesce(max(bi.unit_price), 0), coalesce(max(bi.quantity), 1),
			       (select settings from public.businesses where id = $2), (select timezone from public.businesses where id = $2)
			from public.booking_items bi
			left join public.rental_checkouts rc on rc.booking_id = bi.booking_id
			where bi.booking_id = $1`, b.ID, businessID).Scan(&r.CheckedOutAt, &r.ReturnedAt, &r.OutCondition, &r.ReturnCondition, &dailyPrice, &qty, &rawSettings, &tz)
		if err != nil {
			return b, err
		}
		if r.ReturnedAt != nil && b.EndsAt != nil {
			// A late day costs the daily price × the business's multiplier (e.g. ×2).
			rentals := settings.RentalsFrom(rawSettings)
			r.LateDays = LateDays(*b.EndsAt, *r.ReturnedAt)
			if loc, err := time.LoadLocation(tz); err == nil && rentals.ReturnByEndOfDay {
				r.LateDays = LateDaysByDay(loc, *b.EndsAt, *r.ReturnedAt)
			}
			r.LateFeeSuggested = r.LateDays * rentals.LateFeePerDay(dailyPrice) * int64(qty)
		}
		b.Rental = &r
	}
	return b, nil
}
