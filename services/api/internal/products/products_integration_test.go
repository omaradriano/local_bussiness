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
	sale, err := r.Sell(ctx, biz, []LineInput{{cera.ID, 3}}, "cash", "", "", now)
	if err != nil || sale.Total != 45000 {
		t.Fatalf("sell: %+v %v", sale, err)
	}
	var bad Invalid
	if _, err := r.Sell(ctx, biz, []LineInput{{cera.ID, 8}}, "cash", "", "", now); !errors.As(err, &bad) {
		t.Fatalf("selling more than left: %v", err)
	}
	if p, _ := r.Product(ctx, biz, cera.ID); p.Stock != 7 {
		t.Fatalf("stock after selling 3: %d", p.Stock)
	}
	// A second sale cancelled puts its stock back.
	extra, _ := r.Sell(ctx, biz, []LineInput{{cera.ID, 2}}, "card", "", "", now)
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
}
