package finances

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationSummary(t *testing.T) {
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
	const biz, other = "00000000-0000-0000-0000-0000000000e1", "00000000-0000-0000-0000-0000000000e2"
	const barber, cust, booking = "00000000-0000-0000-0000-0000000000e3", "00000000-0000-0000-0000-0000000000e4", "00000000-0000-0000-0000-0000000000e5"
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Business in Chihuahua (UTC-6). A haircut paid 15 Nov (300) plus a late fee (100); a payment
	// at 23:30 local on 30 Nov (05:30Z on 1 Dec) still belongs to November.
	exec(`insert into public.businesses (id, slug, name, vertical) values ($1, 'fin-a', 'A', 'barbershop'), ($2, 'fin-b', 'B', 'barbershop')`, biz, other)
	exec(`insert into public.staff (id, business_id, name) values ($1, $2, 'Beto')`, barber, biz)
	exec(`insert into public.customers (id, business_id, name, phone) values ($1, $2, 'Ana', '+526140000001')`, cust, biz)
	exec(`insert into public.bookings (id, business_id, customer_id, kind, status, starts_at, ends_at, total, source)
		values ($1, $2, $3, 'service', 'completed', '2026-11-15T17:00:00Z', '2026-11-15T18:00:00Z', 50000, 'admin')`, booking, biz, cust)
	exec(`insert into public.booking_resources (business_id, booking_id, staff_id, during) values ($1, $2, $3, tstzrange('2026-11-15T17:00:00Z', '2026-11-15T18:00:00Z'))`, biz, booking, barber)
	exec(`insert into public.payments (business_id, booking_id, type, method, amount, paid_at) values
		($1, $2, 'balance', 'cash', 30000, '2026-11-15T18:00:00Z'),
		($1, $2, 'balance', 'cash', 20000, '2026-12-01T05:30:00Z'),
		($1, $2, 'late_fee', 'cash', 10000, '2026-11-16T18:00:00Z'),
		($1, $2, 'security_deposit', 'cash', 99900, '2026-11-15T18:00:00Z')`, biz, booking)
	exec(`insert into public.expenses (business_id, spent_on, category, amount) values
		($1, '2026-11-03', 'transport', 40000), ($1, '2026-12-03', 'transport', 1), ($2, '2026-11-03', 'transport', 777)`, biz, other)

	r := Repo{DB: tx}
	// A truck bought 1 Nov: 1,000 down, 12 × 500 from 1 Nov.
	if _, err := r.AddPlan(ctx, biz, Plan{Description: "Camioneta", Category: "asset", PurchasedOn: d("2026-11-01"), DownPayment: 100000,
		InstallmentAmount: 50000, Installments: 12, FirstDue: d("2026-11-01")}, ""); err != nil {
		t.Fatal(err)
	}
	// Beto: 30 % commission, no salary.
	if _, err := r.SetPay(ctx, biz, Pay{StaffID: barber, SalaryPeriod: "monthly", CommissionPercent: 30}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 12, 10, 18, 0, 0, 0, time.UTC)
	s, err := r.Summary(ctx, biz, d("2026-11-01"), d("2026-11-30"), now)
	if err != nil {
		t.Fatal(err)
	}
	part := func(side Side, key string) int64 {
		for _, p := range side.Parts {
			if p.Key == key {
				return p.Amount
			}
		}
		return 0
	}
	// Income: 300 + 200 (local 30 Nov) for services, 100 in fees; the deposit isn't income.
	if s.Income.Total != 60000 || part(s.Income, "bookings") != 50000 || part(s.Income, "fees") != 10000 {
		t.Fatalf("income: %+v", s.Income)
	}
	// Expenses: gasoline 400, truck down payment 1,000 + one installment 500, commission 30 % of 500.
	if part(s.Expenses, "transport") != 40000 || part(s.Expenses, "asset") != 150000 || part(s.Expenses, "commissions") != 15000 {
		t.Fatalf("expenses: %+v", s.Expenses)
	}
	if s.Profit != 60000-(40000+150000+15000) || len(s.Plans) != 1 || s.Plans[0].Remaining != 10*50000 || len(s.Payroll) != 1 {
		t.Fatalf("profit %d plans %+v payroll %+v", s.Profit, s.Plans, s.Payroll)
	}
	// Bookings by status: the November haircut and a cancelled one; December's doesn't count.
	exec(`insert into public.bookings (business_id, customer_id, kind, status, starts_at, ends_at, total, source) values
		($1, $2, 'service', 'cancelled', '2026-11-20T17:00:00Z', '2026-11-20T18:00:00Z', 0, 'admin'),
		($1, $2, 'service', 'confirmed', '2026-12-20T17:00:00Z', '2026-12-20T18:00:00Z', 0, 'admin')`, biz, cust)
	// An overdue rental (due 5 Dec 15:00Z, now is 10 Dec 18:00Z: 6 started days), and another business's.
	var item, rental string
	if err := tx.QueryRow(ctx, `insert into public.catalog_items (business_id, kind, name, price, price_unit, security_deposit, late_fee_per_day) values ($1, 'rental', 'Rotomartillo', 30000, 'day', 100000, 30000) returning id::text`, biz).Scan(&item); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `insert into public.bookings (business_id, customer_id, kind, status, starts_at, ends_at, total, source)
		values ($1, $2, 'rental', 'overdue', '2026-12-01T15:00:00Z', '2026-12-05T15:00:00Z', 120000, 'admin') returning id::text`, biz, cust).Scan(&rental); err != nil {
		t.Fatal(err)
	}
	exec(`insert into public.booking_items (business_id, booking_id, item_id, unit_price) values ($1, $2, $3, 30000)`, biz, rental, item)
	s, err = r.Summary(ctx, biz, d("2026-11-01"), d("2026-11-30"), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Bookings) != 2 || (s.Bookings[0] != Count{"completed", 1}) || (s.Bookings[1] != Count{"cancelled", 1}) {
		t.Fatalf("bookings by status: %+v", s.Bookings)
	}
	if len(s.Overdue) != 1 || s.Overdue[0].BookingID != rental || s.Overdue[0].Customer != "Ana" || s.Overdue[0].Item != "Rotomartillo" || s.Overdue[0].DaysLate != 6 {
		t.Fatalf("overdue: %+v", s.Overdue)
	}
	if err := r.DeleteExpense(ctx, other, "00000000-0000-0000-0000-000000000000"); err != ErrNotFound {
		t.Fatalf("delete missing: %v", err)
	}
}
