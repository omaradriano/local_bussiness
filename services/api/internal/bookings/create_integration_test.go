package bookings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests commit for real (concurrency needs two transactions), so each one builds its own
// businesses with random slugs and deletes them at the end (everything cascades from businesses).

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func suffix() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type fixture struct {
	business, item, staff string
	units                 []string
}

// setup creates one business of the vertical with one item ready to book on Monday 2026-11-02.
func setup(t *testing.T, p *pgxpool.Pool, vertical string, settingsJSON string) fixture {
	t.Helper()
	ctx := context.Background()
	f := fixture{}
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(p.QueryRow(ctx, `insert into public.businesses (slug, name, vertical, settings) values ($1, 'T', $2, $3) returning id::text`,
		"t-"+suffix(), vertical, settingsJSON).Scan(&f.business))
	t.Cleanup(func() { p.Exec(context.Background(), `delete from public.businesses where id = $1`, f.business) })

	switch vertical {
	case "nails":
		must(p.QueryRow(ctx, `insert into public.catalog_items (business_id, kind, name, price, duration_minutes, buffer_minutes)
			values ($1, 'service', 'Gelish', 25000, 60, 15) returning id::text`, f.business).Scan(&f.item))
		must(p.QueryRow(ctx, `insert into public.staff (business_id, name) values ($1, 'Karla') returning id::text`, f.business).Scan(&f.staff))
		_, err := p.Exec(ctx, `insert into public.staff_services (business_id, staff_id, item_id) values ($1, $2, $3)`, f.business, f.staff, f.item)
		must(err)
		_, err = p.Exec(ctx, `insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at) values ($1, $2, 1, '10:00', '18:00')`, f.business, f.staff)
		must(err)
	case "flowers":
		must(p.QueryRow(ctx, `insert into public.catalog_items (business_id, kind, name, price, prep_hours)
			values ($1, 'product', 'Ramo', 65000, 0) returning id::text`, f.business).Scan(&f.item))
		_, err := p.Exec(ctx, `insert into public.business_hours (business_id, weekday, opens_at, closes_at) select $1, d, '09:00', '20:00' from generate_series(0, 6) d`, f.business)
		must(err)
		_, err = p.Exec(ctx, `insert into public.item_options (business_id, item_id, name, type, choices, extra_price, required)
			values ($1, $2, 'Color', 'choice', '["Rojo","Blanco"]', 5000, true)`, f.business, f.item)
		must(err)
	case "tool_rental":
		must(p.QueryRow(ctx, `insert into public.catalog_items (business_id, kind, name, price, price_unit, security_deposit, late_fee_per_day)
			values ($1, 'rental', 'Roto', 30000, 'day', 100000, 30000) returning id::text`, f.business).Scan(&f.item))
		var u string
		must(p.QueryRow(ctx, `insert into public.item_units (business_id, item_id, code) values ($1, $2, 'RM-01') returning id::text`, f.business, f.item).Scan(&u))
		f.units = append(f.units, u)
		_, err := p.Exec(ctx, `insert into public.workflow_steps (business_id, applies_to_kind, actor, moment, to_status, step_type, label)
			values ($1, 'rental', 'business', 'before_status', 'checked_out', 'photo', 'Foto del INE')`, f.business)
		must(err)
	}
	return f
}

var now = time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)

func at(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func cust(phone string) CustomerInput { return CustomerInput{Name: "Ana", Phone: phone} }

// race runs two Creates at the same time and returns their errors.
func race(p *pgxpool.Pool, business string, a, b CreateInput) [2]error {
	var errs [2]error
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, in := range []CreateInput{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, errs[i] = Create(context.Background(), p, business, in, now)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func oneWon(t *testing.T, errs [2]error) {
	t.Helper()
	ok, taken := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, ErrSlotTaken):
			taken++
		default:
			t.Fatalf("unexpected error: %v", e)
		}
	}
	if ok != 1 || taken != 1 {
		t.Fatalf("want exactly one winner and one slot_taken, got %v", errs)
	}
}

func TestIntegrationF3_2_ReservaSimultaneaDevuelve409(t *testing.T) {
	p := pool(t)

	t.Run("misma empleada, mismo horario", func(t *testing.T) {
		f := setup(t, p, "nails", `{"deposit_percent": 30}`)
		in := func(phone string) CreateInput {
			return CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"} // 11:00 local
		}
		oneWon(t, race(p, f.business, in("6141110001"), in("6141110002")))
	})

	t.Run("última unidad de renta", func(t *testing.T) {
		f := setup(t, p, "tool_rental", `{"inspection_minutes": 60}`)
		in := func(phone string) CreateInput {
			return CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-04T15:00:00Z"), Source: "admin"}
		}
		oneWon(t, race(p, f.business, in("6141110003"), in("6141110004")))
	})

	t.Run("último lugar del cupo diario", func(t *testing.T) {
		f := setup(t, p, "flowers", `{"daily_capacity": 1}`)
		in := func(phone string) CreateInput {
			return CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"), Options: map[string]string{"Color": "Rojo"}, Source: "admin"}
		}
		oneWon(t, race(p, f.business, in("6141110005"), in("6141110006")))
	})
}

func TestIntegrationCreateBooking(t *testing.T) {
	p := pool(t)
	ctx := context.Background()

	t.Run("cita: anticipo, apartado que vence, empleada asignada, margen ocupado", func(t *testing.T) {
		f := setup(t, p, "nails", `{"deposit_percent": 30, "hold_hours": 4}`)
		b, existed, err := Create(ctx, p, f.business, CreateInput{Customer: cust("614 222 0001"), ItemID: f.item,
			StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin", IdempotencyKey: "key-cita-0001"}, now)
		if err != nil || existed {
			t.Fatalf("create: %v existed=%v", err, existed)
		}
		if b.Status != "pending_deposit" || b.Total != 25000 || b.DepositRequired != 7500 || b.HoldExpiresAt == nil ||
			!b.HoldExpiresAt.Equal(now.Add(4*time.Hour)) || len(b.Staff) != 1 || b.Customer.Phone != "+526142220001" {
			t.Fatalf("booking: %+v", b)
		}
		// Same Idempotency-Key: same booking, not a second one.
		again, existed, err := Create(ctx, p, f.business, CreateInput{Customer: cust("614 222 0001"), ItemID: f.item,
			StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin", IdempotencyKey: "key-cita-0001"}, now)
		if err != nil || !existed || again.ID != b.ID {
			t.Fatalf("idempotent retry: %v existed=%v id=%s", err, existed, again.ID)
		}
		// 12:00 local starts during the 15 min buffer of the 11:00–12:00 appointment.
		if _, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6142220002"), ItemID: f.item,
			StartsAt: at("2026-11-02T18:00:00Z"), Source: "admin"}, now); !errors.Is(err, ErrSlotTaken) {
			t.Fatalf("slot inside the buffer: %v", err)
		}
		// Off-shift time: not a valid slot.
		if _, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6142220003"), ItemID: f.item,
			StartsAt: at("2026-11-02T14:00:00Z"), Source: "admin"}, now); !errors.Is(err, ErrSlotTaken) {
			t.Fatalf("off-shift: %v", err)
		}
	})

	t.Run("pedido: opción obligatoria, costo extra, entrega por zona", func(t *testing.T) {
		f := setup(t, p, "flowers", `{"deposit_percent": 50, "delivery_zones": [{"name": "Centro", "fee": 6000}]}`)
		base := CreateInput{Customer: cust("6143330001"), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"), Source: "admin"}
		var inv Invalid
		if _, _, err := Create(ctx, p, f.business, base, now); !errors.As(err, &inv) || inv.Field != "options.Color" {
			t.Fatalf("missing required option: %v", err)
		}
		in := base
		in.Options = map[string]string{"Color": "Azul"}
		if _, _, err := Create(ctx, p, f.business, in, now); !errors.As(err, &inv) {
			t.Fatalf("invalid choice: %v", err)
		}
		in.Options = map[string]string{"Color": "Rojo"}
		in.Fulfillment, in.DeliveryAddress, in.DeliveryZone = "delivery", "Av. Juárez 100", "Norte"
		if _, _, err := Create(ctx, p, f.business, in, now); !errors.As(err, &inv) || inv.Field != "delivery_zone" {
			t.Fatalf("unknown zone: %v", err)
		}
		in.DeliveryZone = "Centro"
		b, _, err := Create(ctx, p, f.business, in, now)
		if err != nil || b.Total != 65000+5000+6000 || b.DepositRequired != 38000 || b.EndsAt != nil || *b.Fulfillment != "delivery" {
			t.Fatalf("order: %+v %v", b, err)
		}
	})

	t.Run("renta: días cobrados, depósito aparte, unidad asignada, pasos copiados", func(t *testing.T) {
		f := setup(t, p, "tool_rental", `{}`)
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6144440001"), ItemID: f.item,
			StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-04T16:00:00Z"), Source: "admin"}, now)
		if err != nil || b.Status != "confirmed" || b.Total != 3*30000 || len(b.Units) != 1 || b.Units[0].Name != "RM-01" {
			t.Fatalf("rental: %+v %v", b, err)
		}
		if b.Items[0].Options["security_deposit"].(float64) != 100000 {
			t.Fatalf("security deposit: %+v", b.Items[0].Options)
		}
		var steps int
		p.QueryRow(ctx, `select count(*) from public.booking_steps where booking_id = $1`, b.ID).Scan(&steps)
		if steps != 1 {
			t.Fatalf("steps copied = %d", steps)
		}
		if _, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6144440002"), ItemID: f.item,
			StartsAt: at("2026-11-03T15:00:00Z"), EndsAt: at("2026-11-05T15:00:00Z"), Source: "admin"}, now); !errors.Is(err, ErrSlotTaken) {
			t.Fatalf("overlapping rental: %v", err)
		}
	})

	t.Run("mismo teléfono es el mismo cliente", func(t *testing.T) {
		f := setup(t, p, "tool_rental", `{}`)
		b1, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("614 555 0001"), ItemID: f.item,
			StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-03T15:00:00Z"), Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		b2, _, err := Create(ctx, p, f.business, CreateInput{Customer: CustomerInput{Name: "Otra", Phone: "+52 614 555 0001"}, ItemID: f.item,
			StartsAt: at("2026-11-10T15:00:00Z"), EndsAt: at("2026-11-11T15:00:00Z"), Source: "admin"}, now)
		if err != nil || b2.Customer.ID != b1.Customer.ID || b2.Customer.Name != "Ana" {
			t.Fatalf("customer reuse: %+v %v", b2.Customer, err)
		}
	})
}

func TestIntegrationListBookings(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{}`)
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6146660001"), ItemID: f.item,
		StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-04T15:00:00Z"), Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	win := func(from, to string) ListFilter { return ListFilter{From: *at(from), To: *at(to), OnlyActive: true} }

	// A rental spanning the window is listed even if it started before it.
	got, err := List(ctx, p, f.business, win("2026-11-03T00:00:00Z", "2026-11-03T23:59:00Z"))
	if err != nil || len(got) != 1 || got[0].ID != b.ID || got[0].Units[0] != "RM-01" || got[0].Items != "1 × Roto" || got[0].CustomerName != "Ana" {
		t.Fatalf("overlapping: %+v %v", got, err)
	}
	if got, _ := List(ctx, p, f.business, win("2026-11-05T00:00:00Z", "2026-11-06T00:00:00Z")); len(got) != 0 {
		t.Fatalf("after it ended: %+v", got)
	}
	if got, _ := List(ctx, p, f.business, ListFilter{From: *at("2026-11-01T00:00:00Z"), To: *at("2026-11-30T00:00:00Z"), Kind: "service"}); len(got) != 0 {
		t.Fatalf("kind filter: %+v", got)
	}
	// Cancelled rentals leave the active list but stay in "all".
	if _, err := p.Exec(ctx, `update public.bookings set status = 'cancelled' where id = $1`, b.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := List(ctx, p, f.business, win("2026-11-01T00:00:00Z", "2026-11-30T00:00:00Z")); len(got) != 0 {
		t.Fatalf("cancelled still active: %+v", got)
	}
	all := win("2026-11-01T00:00:00Z", "2026-11-30T00:00:00Z")
	all.OnlyActive = false
	if got, _ := List(ctx, p, f.business, all); len(got) != 1 || got[0].Status != "cancelled" {
		t.Fatalf("all statuses: %+v", got)
	}
}
