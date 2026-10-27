package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("booking not found")

type Customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
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
}

// Get returns one booking of the business with its customer, lines and assigned staff/units.
func Get(ctx context.Context, db store.DB, businessID, id string) (Booking, error) {
	var b Booking
	var items, staff, units []byte
	err := db.QueryRow(ctx, `
		select b.id::text, b.kind, b.status, b.starts_at, b.ends_at,
		       c.id::text, c.name, c.phone,
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
		&b.ID, &b.Kind, &b.Status, &b.StartsAt, &b.EndsAt, &b.Customer.ID, &b.Customer.Name, &b.Customer.Phone,
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
	return b, nil
}
