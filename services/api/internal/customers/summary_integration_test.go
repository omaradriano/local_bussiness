package customers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationCustomerSummary(t *testing.T) {
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
	const biz, ana = "00000000-0000-0000-0000-0000000000c1", "00000000-0000-0000-0000-0000000000c2"
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`insert into public.businesses (id, slug, name, vertical) values ($1, 'sum-a', 'A', 'flowers')`, biz)
	exec(`insert into public.customers (id, business_id, name, phone) values ($1, $2, 'Ana', '+526140000001')`, ana, biz)
	order := func(status, starts, recipient string, total, paid int64) string {
		t.Helper()
		var id string
		if err := tx.QueryRow(ctx, `insert into public.bookings (business_id, customer_id, kind, status, starts_at, total, source, recipient_name, recipient_phone)
			values ($1, $2, 'product', $3, $4, $5, 'admin', nullif($6, ''), null) returning id::text`, biz, ana, status, starts, total, recipient).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if paid > 0 {
			exec(`insert into public.payments (business_id, booking_id, type, method, amount) values ($1, $2, 'balance', 'cash', $3)`, biz, id, paid)
		}
		return id
	}
	// Three orders for Mamá (one written differently), one for Lucía; one cancelled that doesn't count.
	order("completed", "2026-02-14T18:00:00Z", "Mamá", 60000, 60000)
	order("completed", "2026-05-10T18:00:00Z", " mamá ", 80000, 50000) // on credit: owes 300
	order("confirmed", "2026-11-20T18:00:00Z", "Lucía", 40000, 20000)  // owes 200
	last := order("confirmed", "2026-12-24T18:00:00Z", "Mamá", 50000, 0)
	order("cancelled", "2026-12-25T18:00:00Z", "Lucía", 90000, 0)
	// A product paid at the counter with the order counts as spent; one left on account isn't paid yet.
	exec(`insert into public.sales (business_id, booking_id, method, total, cost_total) values ($1, $2, 'cash', 9000, 0), ($1, $2, 'on_account', 5000, 0)`, biz, last)

	s, err := Repo{DB: tx}.Summary(ctx, biz, ana)
	if err != nil {
		t.Fatal(err)
	}
	if s.Orders != 4 || s.Spent != 60000+50000+20000+9000 || s.Due != 30000+20000+50000+5000 ||
		s.FirstAt == nil || s.FirstAt.Month() != 2 || s.LastAt == nil || s.LastAt.Month() != 12 {
		t.Fatalf("summary: %+v", s)
	}
	if len(s.Recipients) != 2 || s.Recipients[0].Name != "Mamá" || s.Recipients[0].Count != 3 || s.Recipients[1].Name != "Lucía" || s.Recipients[1].Count != 1 {
		t.Fatalf("recipients: %+v", s.Recipients)
	}
}

func TestIntegrationCustomerDates(t *testing.T) {
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
	const biz, ana = "00000000-0000-0000-0000-0000000000c3", "00000000-0000-0000-0000-0000000000c4"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'dates-a', 'A', 'flowers')`, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `insert into public.customers (id, business_id, name, phone) values ($1, $2, 'Ana', '+526140000001')`, ana, biz); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	if _, err := r.AddDate(ctx, biz, ana, Date{Label: "x", Month: 4, Day: 31}); err == nil {
		t.Fatal("31 April")
	}
	soon, err := r.AddDate(ctx, biz, ana, Date{Label: "  Cumpleaños   de su esposa ", Month: 11, Day: 14})
	if err != nil || soon.Label != "Cumpleaños de su esposa" {
		t.Fatalf("add: %+v %v", soon, err)
	}
	if _, err := r.AddDate(ctx, biz, ana, Date{Label: "Aniversario", Month: 6, Day: 2}); err != nil {
		t.Fatal(err)
	}
	if list, err := r.Dates(ctx, biz, ana); err != nil || len(list) != 2 || list[0].Label != "Aniversario" {
		t.Fatalf("dates by month: %+v %v", list, err)
	}
	up, err := r.UpcomingDates(ctx, biz, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), 7)
	if err != nil || len(up) != 1 || up[0].ID != soon.ID || up[0].DaysLeft != 3 || up[0].CustomerName != "Ana" {
		t.Fatalf("upcoming: %+v %v", up, err)
	}
	if err := r.DeleteDate(ctx, biz, ana, soon.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteDate(ctx, biz, ana, soon.ID); err != ErrNotFound {
		t.Fatalf("delete twice: %v", err)
	}
}
