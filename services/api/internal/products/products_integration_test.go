package products

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/finances"
)

func TestIntegrationProductsFlow(t *testing.T) {
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
	const biz = "00000000-0000-0000-0000-0000000000d1"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'prod-a', 'A', 'barbershop')`, biz); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }
	now := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)

	cost := int64(7000)
	cera, err := r.Create(ctx, biz, Product{Name: "Cera", Price: 15000, Cost: &cost, Stock: 0})
	if err != nil {
		t.Fatal(err)
	}
	// Buy 10 at 70: stock 10, counts as merchandise expense.
	if cera, err = r.Purchase(ctx, biz, cera.ID, 10, 7000, day("2026-11-02"), ""); err != nil || cera.Stock != 10 {
		t.Fatalf("purchase: %+v %v", cera, err)
	}
	var bad Invalid
	if _, err := r.Purchase(ctx, biz, cera.ID, 5, 7000, time.Now().AddDate(0, 0, 3), ""); !errors.As(err, &bad) || bad["purchased_on"] == "" {
		t.Fatalf("purchase dated in the future: %v", err)
	}
	sale, err := r.Sell(ctx, biz, SaleInput{Lines: []LineInput{{ProductID: cera.ID, Quantity: 3}}, Method: "cash", Retail: true}, "", now)
	if err != nil || sale.Total != 45000 {
		t.Fatalf("sell: %+v %v", sale, err)
	}
	if _, err := r.Sell(ctx, biz, SaleInput{Lines: []LineInput{{ProductID: cera.ID, Quantity: 8}}, Method: "cash", Retail: true}, "", now); !errors.As(err, &bad) {
		t.Fatalf("selling more than left: %v", err)
	}
	if p, _ := r.Product(ctx, biz, cera.ID); p.Stock != 7 {
		t.Fatalf("stock after selling 3: %d", p.Stock)
	}
	// A second sale cancelled puts its stock back.
	extra, _ := r.Sell(ctx, biz, SaleInput{Lines: []LineInput{{ProductID: cera.ID, Quantity: 2}}, Method: "card", Retail: true}, "", now)
	if err := r.CancelSale(ctx, biz, extra.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := r.Product(ctx, biz, cera.ID); p.Stock != 7 {
		t.Fatalf("stock after cancel: %d", p.Stock)
	}

	start, end, _ := r.Bounds(ctx, biz, day("2026-11-01"), day("2026-11-30"))
	st, err := r.StatsFor(ctx, biz, start, end, day("2026-11-01"), day("2026-11-30"))
	if err != nil || st.Revenue != 45000 || st.Cost != 21000 || st.Profit != 24000 || st.Units != 3 || st.Purchases != 70000 || st.Sales != 1 {
		t.Fatalf("stats: %+v %v", st, err)
	}
	// Finances: sales are income, merchandise an expense.
	s, err := finances.Repo{DB: tx}.Summary(ctx, biz, day("2026-11-01"), day("2026-11-30"), now)
	if err != nil || s.Income.Total != 45000 || s.Expenses.Total != 70000 {
		t.Fatalf("summary: %+v %+v %v", s.Income, s.Expenses, err)
	}

	// Quick sale: a catalog item, a free amount and a product, to a walk-in with a name.
	var corte string
	if err := tx.QueryRow(ctx, `insert into public.catalog_items (business_id, kind, name, price, duration_minutes)
		values ($1, 'service', 'Corte', 20000, 30) returning id::text`, biz).Scan(&corte); err != nil {
		t.Fatal(err)
	}
	free := int64(5000)
	quick := SaleInput{Lines: []LineInput{{ItemID: corte, Quantity: 1}, {Name: "Propina", Price: &free, Quantity: 1}, {ProductID: cera.ID, Quantity: 1}},
		Method: "cash", CustomerName: "Don Beto", Quick: true}
	later := now.Add(time.Minute) // after the first sale, so the list (newest first) has a fixed order
	if _, err := r.Sell(ctx, biz, quick, "", later); !errors.As(err, &bad) {
		t.Fatalf("products without productSales: %v", err)
	}
	quick.Retail = true
	q, err := r.Sell(ctx, biz, quick, "", later)
	if err != nil || q.Total != 20000+5000+15000 || *q.CustomerName != "Don Beto" {
		t.Fatalf("quick sale: %+v %v", q, err)
	}
	if p, _ := r.Product(ctx, biz, cera.ID); p.Stock != 6 {
		t.Fatalf("stock after the quick sale: %d", p.Stock)
	}
	list, err := r.Sales(ctx, biz, start, end)
	if err != nil || len(list) != 2 || list[0].ID != q.ID || len(list[0].Lines) != 3 || list[0].Lines[0].Name != "Cera" || list[0].Lines[2].Name != "Propina" {
		t.Fatalf("sales: %+v %v", list, err)
	}
	if err := r.CancelSale(ctx, biz, q.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := r.Product(ctx, biz, cera.ID); p.Stock != 7 {
		t.Fatalf("stock after cancelling the quick sale: %d", p.Stock)
	}

	// Q2: the extras of an order already handed over (or cancelled) stay; deleting one would put back
	// stock that left with the order and leave its payment without a sale.
	var cust, order string
	if err := tx.QueryRow(ctx, `insert into public.customers (business_id, name, phone) values ($1, 'Ana', '+526140000001') returning id::text`, biz).Scan(&cust); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `insert into public.bookings (business_id, customer_id, kind, status, starts_at, source, total)
		values ($1, $2, 'product', 'confirmed', $3, 'admin', 75000) returning id::text`, biz, cust, now).Scan(&order); err != nil {
		t.Fatal(err)
	}
	extraOf := func() Sale {
		s, err := r.Sell(ctx, biz, SaleInput{BookingID: order, Lines: []LineInput{{ProductID: cera.ID, Quantity: 1}}, Method: "cash", Retail: true}, "", now)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	// While the order is open, a mistaken extra can still go.
	if err := r.CancelSale(ctx, biz, extraOf().ID); err != nil {
		t.Fatalf("extra of an open order: %v", err)
	}
	kept := extraOf()
	if _, err := tx.Exec(ctx, `update public.bookings set status = 'completed' where id = $1`, order); err != nil {
		t.Fatal(err)
	}
	if err := r.CancelSale(ctx, biz, kept.ID); !errors.As(err, &bad) || bad["sale"] == "" {
		t.Fatalf("extra of a delivered order: %v", err)
	}
	if p, _ := r.Product(ctx, biz, cera.ID); p.Stock != 6 {
		t.Fatalf("stock must not come back: %d", p.Stock)
	}
	// The list says so, so the app hides the delete button.
	list, err = r.Sales(ctx, biz, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.ID == kept.ID && !s.Locked {
			t.Fatal("extra of a delivered order must be locked")
		}
	}
}

func TestIntegrationSaleDiscount(t *testing.T) {
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
	const biz = "00000000-0000-0000-0000-0000000000d3"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'prod-disc', 'A', 'flowers')`, biz); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }
	now := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	cost := int64(5000)
	choco, err := r.Create(ctx, biz, Product{Name: "Chocolates", Price: 15000, Cost: &cost, Stock: 5})
	if err != nil {
		t.Fatal(err)
	}
	free := int64(10000)
	lines := []LineInput{{ProductID: choco.ID, Quantity: 2}, {Name: "Ramo", Price: &free, Quantity: 1}}
	var bad Invalid
	if _, err := r.Sell(ctx, biz, SaleInput{Lines: lines, Method: "cash", Retail: true, Quick: true,
		Discount: Discount{Type: "amount", Value: 40001}}, "", now); !errors.As(err, &bad) || bad["discount"] == "" {
		t.Fatalf("discount over the total: %v", err)
	}
	// 2 × 150 + 100 = 400, 25 % off: 300 charged, 100 off.
	sale, err := r.Sell(ctx, biz, SaleInput{Lines: lines, Method: "cash", Retail: true, Quick: true, Discount: Discount{Type: "percent", Value: 25}}, "", now)
	if err != nil || sale.Total != 30000 || sale.Discount != 10000 {
		t.Fatalf("sale with discount: %+v %v", sale, err)
	}
	// The discount comes off each part in proportion: products 300 → 225, counter 100 → 75.
	start, end, _ := r.Bounds(ctx, biz, day("2026-11-01"), day("2026-11-30"))
	if st, err := r.StatsFor(ctx, biz, start, end, day("2026-11-01"), day("2026-11-30")); err != nil || st.Revenue != 22500 {
		t.Fatalf("stats with discount: %+v %v", st, err)
	}
	s, err := finances.Repo{DB: tx}.Summary(ctx, biz, day("2026-11-01"), day("2026-11-30"), now)
	if err != nil || s.Income.Total != 30000 {
		t.Fatalf("income with discount: %+v %v", s.Income, err)
	}
	for _, p := range s.Income.Parts {
		if (p.Key == "products" && p.Amount != 22500) || (p.Key == "counter" && p.Amount != 7500) {
			t.Fatalf("parts: %+v", s.Income.Parts)
		}
	}
}

func TestIntegrationAverageCostAndMovements(t *testing.T) {
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
	const biz = "00000000-0000-0000-0000-0000000000d4"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'prod-avg', 'A', 'flowers')`, biz); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }
	cost := int64(6000)
	p, err := r.Create(ctx, biz, Product{Name: "Chocolates", Price: 9000, Cost: &cost, Stock: 10})
	if err != nil {
		t.Fatal(err)
	}
	// 10 at 60 on hand + 10 bought at 30 = 20 at 45, not 30.
	if p, err = r.Purchase(ctx, biz, p.ID, 10, 3000, day("2026-11-02"), ""); err != nil || *p.Cost != 4500 || p.Stock != 20 {
		t.Fatalf("average: %+v %v", p, err)
	}
	// 20 at 45 + 1 at 50 = 45.24 → 45.24 rounded to centavos.
	if p, err = r.Purchase(ctx, biz, p.ID, 1, 5000, day("2026-11-03"), ""); err != nil || *p.Cost != 4524 {
		t.Fatalf("rounded average: %+v %v", *p.Cost, err)
	}
	if _, err := r.Sell(ctx, biz, SaleInput{Lines: []LineInput{{ProductID: p.ID, Quantity: 2}}, Method: "cash", Retail: true}, "", time.Date(2026, 11, 4, 18, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	// History, newest first: the sale (−2 at its price), then both purchases (+ at their cost).
	moves, err := r.Movements(ctx, biz, p.ID)
	if err != nil || len(moves) != 3 || moves[0].Kind != "sale" || moves[0].Quantity != -2 || moves[0].Amount != 9000 ||
		moves[1].Kind != "purchase" || moves[1].Quantity != 1 || moves[1].Amount != 5000 || moves[2].Quantity != 10 {
		t.Fatalf("movements: %+v %v", moves, err)
	}
}

func TestIntegrationStockAdjustments(t *testing.T) {
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
	const biz = "00000000-0000-0000-0000-0000000000d5"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'prod-adj', 'A', 'flowers')`, biz); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	cost := int64(4000)
	p, err := r.Create(ctx, biz, Product{Name: "Globo", Price: 9000, Cost: &cost, Stock: 10})
	if err != nil {
		t.Fatal(err)
	}
	var bad Invalid
	if _, err := r.Adjust(ctx, biz, p.ID, Adjustment{Reason: "waste", Quantity: 11}, ""); !errors.As(err, &bad) || bad["quantity"] == "" {
		t.Fatalf("waste over the stock: %v", err)
	}
	if _, err := r.Adjust(ctx, biz, p.ID, Adjustment{Reason: "count", Counted: ptrInt(10)}, ""); !errors.As(err, &bad) || bad["counted"] == "" {
		t.Fatalf("count that changes nothing: %v", err)
	}
	// 3 popped (waste), then counted 6 where 7 were left: −1.
	if p, err = r.Adjust(ctx, biz, p.ID, Adjustment{Reason: "waste", Quantity: 3, Note: "Se reventaron"}, ""); err != nil || p.Stock != 7 {
		t.Fatalf("waste: %+v %v", p, err)
	}
	if p, err = r.Adjust(ctx, biz, p.ID, Adjustment{Reason: "count", Counted: ptrInt(6)}, ""); err != nil || p.Stock != 6 {
		t.Fatalf("count: %+v %v", p, err)
	}
	moves, err := r.Movements(ctx, biz, p.ID)
	if err != nil || len(moves) != 2 || moves[0].Kind != "count" || moves[0].Quantity != -1 || moves[1].Kind != "waste" || moves[1].Quantity != -3 || moves[1].Note == nil {
		t.Fatalf("movements: %+v %v", moves, err)
	}
	// Waste is valued at cost in the stats (3 × 40); a count difference isn't waste.
	now := time.Now()
	st, err := r.StatsFor(ctx, biz, now.Add(-time.Hour), now.Add(time.Hour), now, now)
	if err != nil || st.Waste != 12000 {
		t.Fatalf("waste in stats: %+v %v", st.Waste, err)
	}
}

func ptrInt(n int) *int { return &n }

func TestIntegrationMinStockAndDelete(t *testing.T) {
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
	const biz = "00000000-0000-0000-0000-0000000000d6"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'prod-min', 'A', 'flowers')`, biz); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	cost := int64(1000)
	// Low stock follows each product's minimum: 8 with a minimum of 10 is low; 2 with 0 isn't.
	globo, _ := r.Create(ctx, biz, Product{Name: "Globo", Price: 5000, Cost: &cost, Stock: 8, MinStock: 10})
	tarjeta, _ := r.Create(ctx, biz, Product{Name: "Tarjeta", Price: 3000, Cost: &cost, Stock: 2, MinStock: 0})
	now := time.Now()
	st, err := r.StatsFor(ctx, biz, now.Add(-time.Hour), now.Add(time.Hour), now, now)
	if err != nil || len(st.LowStock) != 1 || st.LowStock[0].ID != globo.ID {
		t.Fatalf("low stock: %+v %v", st.LowStock, err)
	}
	// Without history it's gone; with a sale it's archived and its sale keeps its name.
	if archived, err := r.Delete(ctx, biz, tarjeta.ID); err != nil || archived {
		t.Fatalf("delete without history: %v %v", archived, err)
	}
	sale, err := r.Sell(ctx, biz, SaleInput{Lines: []LineInput{{ProductID: globo.ID, Quantity: 1}}, Method: "cash", Retail: true}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if archived, err := r.Delete(ctx, biz, globo.ID); err != nil || !archived {
		t.Fatalf("delete with a sale: %v %v", archived, err)
	}
	if list, _ := r.Products(ctx, biz, true); len(list) != 0 {
		t.Fatalf("archived products are hidden: %+v", list)
	}
	if _, err := r.Product(ctx, biz, globo.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archived product by id: %v", err)
	}
	if sales, _ := r.Sales(ctx, biz, now.Add(-time.Hour), now.Add(time.Hour)); len(sales) != 1 || sales[0].ID != sale.ID || sales[0].Lines[0].Name != "Globo" {
		t.Fatalf("the sale keeps the product: %+v", sales)
	}
	if _, err := r.Sell(ctx, biz, SaleInput{Lines: []LineInput{{ProductID: globo.ID, Quantity: 1}}, Method: "cash", Retail: true}, "", now); err == nil {
		t.Fatal("an archived product can't be sold")
	}
}
