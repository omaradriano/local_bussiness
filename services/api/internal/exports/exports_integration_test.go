package exports

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationExports(t *testing.T) {
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
	const biz, cust, item, booking, product, sale = "00000000-0000-0000-0000-0000000000e7", "00000000-0000-0000-0000-0000000000e8",
		"00000000-0000-0000-0000-0000000000e9", "00000000-0000-0000-0000-0000000000ea", "00000000-0000-0000-0000-0000000000eb", "00000000-0000-0000-0000-0000000000ec"
	for _, q := range []string{
		`insert into public.businesses (id, slug, name, vertical) values ('` + biz + `', 'export-a', 'A', 'tool_rental')`,
		`insert into public.customers (id, business_id, name, phone) values ('` + cust + `', '` + biz + `', 'Ana', '+526140000009')`,
		`insert into public.customers (business_id, name, phone, walk_in) values ('` + biz + `', 'Cliente casual', '+10000000000', true)`,
		`insert into public.catalog_items (id, business_id, kind, name, price, price_unit, security_deposit, late_fee_per_day) values ('` + item + `', '` + biz + `', 'rental', 'Rotomartillo', 30000, 'day', 100000, 30000)`,
		`insert into public.bookings (id, business_id, customer_id, kind, status, starts_at, ends_at, total, source) values ('` + booking + `', '` + biz + `', '` + cust + `', 'rental', 'completed', '2026-10-05T15:00:00Z', '2026-10-07T15:00:00Z', 60000, 'admin')`,
		`insert into public.booking_items (business_id, booking_id, item_id, quantity, unit_price) values ('` + biz + `', '` + booking + `', '` + item + `', 2, 30000)`,
		`insert into public.payments (business_id, booking_id, type, method, amount) values ('` + biz + `', '` + booking + `', 'advance', 'cash', 20000), ('` + biz + `', '` + booking + `', 'late_fee', 'cash', 5000), ('` + biz + `', '` + booking + `', 'security_deposit', 'cash', 100000)`,
		`insert into public.expenses (business_id, spent_on, category, amount) values ('` + biz + `', '2026-10-03', 'transport', 40000), ('` + biz + `', '2026-11-03', 'rent', 1)`,
		`insert into public.retail_products (id, business_id, name, price, cost, stock) values ('` + product + `', '` + biz + `', 'Brocas', 15000, 7000, 5)`,
		`insert into public.sales (id, business_id, sold_at, method, total, cost_total) values ('` + sale + `', '` + biz + `', '2026-10-10T18:00:00Z', 'cash', 30000, 14000)`,
		`insert into public.sale_lines (business_id, sale_id, product_id, quantity, unit_price, unit_cost) values ('` + biz + `', '` + sale + `', '` + product + `', 2, 15000, 7000)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	r := Repo{DB: tx}
	oct := &Period{From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}

	got, err := r.Rows(ctx, biz, "customers", nil)
	if cs := got.([]Customer); err != nil || len(cs) != 1 || cs[0].Name != "Ana" {
		t.Fatalf("customers (no walk-in): %+v %v", got, err)
	}
	got, err = r.Rows(ctx, biz, "bookings", oct)
	if bs := got.([]Booking); err != nil || len(bs) != 1 || bs[0].Items != "Rotomartillo × 2" || bs[0].Paid != 20000 || bs[0].Due != 40000 || bs[0].Extra != 5000 {
		t.Fatalf("bookings: %+v %v", got, err)
	}
	got, err = r.Rows(ctx, biz, "expenses", oct)
	if es := got.([]Expense); err != nil || len(es) != 1 || es[0].Amount != 40000 {
		t.Fatalf("expenses of october: %+v %v", got, err)
	}
	if got, _ = r.Rows(ctx, biz, "expenses", nil); len(got.([]Expense)) != 2 {
		t.Fatalf("all expenses: %+v", got)
	}
	got, err = r.Rows(ctx, biz, "sales", oct)
	if ss := got.([]Sale); err != nil || len(ss) != 1 || ss[0].Products != "2 × Brocas" || ss[0].Cost != 14000 {
		t.Fatalf("sales: %+v %v", got, err)
	}
	got, err = r.Rows(ctx, biz, "catalog", nil)
	if is := got.([]CatalogItem); err != nil || len(is) != 1 || is[0].LateFeePerDay == nil || *is[0].LateFeePerDay != 30000 {
		t.Fatalf("catalog: %+v %v", got, err)
	}
	got, err = r.Rows(ctx, biz, "products", nil)
	if ps := got.([]Product); err != nil || len(ps) != 1 || ps[0].Stock != 5 {
		t.Fatalf("products: %+v %v", got, err)
	}
}
