package availability

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// End to end on a real Postgres: data written the way the app stores it, read back through Answer.
func TestIntegrationAnswerReadsTheDatabase(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	const (
		nails, flowers, tools     = "00000000-0000-0000-0000-0000000000a1", "00000000-0000-0000-0000-0000000000a2", "00000000-0000-0000-0000-0000000000a3"
		karla, gelish, ramo, roto = "00000000-0000-0000-0000-0000000000b1", "00000000-0000-0000-0000-0000000000c1", "00000000-0000-0000-0000-0000000000c2", "00000000-0000-0000-0000-0000000000c3"
		rm1, rm2, cust            = "00000000-0000-0000-0000-0000000000d1", "00000000-0000-0000-0000-0000000000d2", "00000000-0000-0000-0000-0000000000e1"
		cust3                     = "00000000-0000-0000-0000-0000000000e3"
	)
	for _, q := range []string{
		`insert into public.businesses (id, slug, name, vertical, settings) values
		   ('` + nails + `', 'av-unas', 'U', 'nails', '{"slot_minutes": 60}'),
		   ('` + flowers + `', 'av-flores', 'F', 'flowers', '{"daily_capacity": 1}'),
		   ('` + tools + `', 'av-renta', 'R', 'tool_rental', '{"inspection_minutes": 60}')`,
		`insert into public.staff (id, business_id, name) values ('` + karla + `', '` + nails + `', 'Karla')`,
		`insert into public.catalog_items (id, business_id, kind, name, price, duration_minutes) values ('` + gelish + `', '` + nails + `', 'service', 'Gelish', 25000, 60)`,
		`insert into public.catalog_items (id, business_id, kind, name, price, prep_hours) values ('` + ramo + `', '` + flowers + `', 'product', 'Ramo', 65000, 0)`,
		`insert into public.catalog_items (id, business_id, kind, name, price, price_unit, security_deposit, late_fee_per_day) values ('` + roto + `', '` + tools + `', 'rental', 'Roto', 30000, 'day', 1, 1)`,
		`insert into public.staff_services (business_id, staff_id, item_id) values ('` + nails + `', '` + karla + `', '` + gelish + `')`,
		// Monday 10-13 for Karla; flowers open every day 9-20.
		`insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at) values ('` + nails + `', '` + karla + `', 1, '10:00', '13:00')`,
		`insert into public.business_hours (business_id, weekday, opens_at, closes_at) select '` + flowers + `', d, '09:00', '20:00' from generate_series(0, 6) d`,
		`insert into public.item_units (id, business_id, item_id, code) values ('` + rm1 + `', '` + tools + `', '` + roto + `', 'RM-01'), ('` + rm2 + `', '` + tools + `', '` + roto + `', 'RM-02')`,
		`insert into public.customers (id, business_id, name, phone) values ('` + cust + `', '` + nails + `', 'Ana', '+526140000001'), ('` + cust3 + `', '` + tools + `', 'Beto', '+526140000003')`,
		// Karla has a booking Monday 11:00-12:00 (Chihuahua = UTC-6).
		`insert into public.bookings (id, business_id, customer_id, kind, status, starts_at, ends_at, source) values
		   ('00000000-0000-0000-0000-0000000000f1', '` + nails + `', '` + cust + `', 'service', 'confirmed', '2026-11-02 17:00Z', '2026-11-02 18:00Z', 'admin')`,
		`insert into public.booking_resources (business_id, booking_id, staff_id, during) values
		   ('` + nails + `', '00000000-0000-0000-0000-0000000000f1', '` + karla + `', '[2026-11-02 17:00Z, 2026-11-02 18:00Z)')`,
		// RM-01 is out Monday to Wednesday.
		`insert into public.bookings (id, business_id, customer_id, kind, status, starts_at, ends_at, source) values
		   ('00000000-0000-0000-0000-0000000000f3', '` + tools + `', '` + cust3 + `', 'rental', 'checked_out', '2026-11-02 15:00Z', '2026-11-04 15:00Z', 'admin')`,
		`insert into public.booking_resources (business_id, booking_id, item_unit_id, during) values
		   ('` + tools + `', '00000000-0000-0000-0000-0000000000f3', '` + rm1 + `', '[2026-11-02 15:00Z, 2026-11-04 15:00Z)')`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	l := Loader{DB: tx}
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)

	svc, err := l.Answer(ctx, nails, Request{ItemID: gelish, Date: "2026-11-02"}, now)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range svc.Slots {
		got = append(got, s.Start.In(chih).Format("15:04"))
	}
	if !equal(got, []string{"10:00", "12:00"}) {
		t.Fatalf("service slots = %v (11:00 is booked)", got)
	}

	prod, err := l.Answer(ctx, flowers, Request{ItemID: ramo, From: "2026-11-02", To: "2026-11-03"}, now)
	if err != nil || len(prod.Days) != 2 || !prod.Days[0].Available || prod.Days[0].Remaining != 1 {
		t.Fatalf("product days = %+v %v", prod.Days, err)
	}

	rent, err := l.Answer(ctx, tools, Request{ItemID: roto, From: "2026-11-03T15:00:00Z", To: "2026-11-05T15:00:00Z", Quantity: 2}, now)
	if err != nil || rent.Available || !equal(rent.FreeUnits, []string{rm2}) {
		t.Fatalf("rental = %+v %v", rent, err)
	}

	if _, err := l.Answer(ctx, flowers, Request{ItemID: gelish, Date: "2026-11-02"}, now); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("item of another business: %v", err)
	}
	var bad BadRequest
	if _, err := l.Answer(ctx, nails, Request{ItemID: gelish, Date: "mañana"}, now); !errors.As(err, &bad) || bad.Field != "date" {
		t.Fatalf("bad date: %v", err)
	}
}
