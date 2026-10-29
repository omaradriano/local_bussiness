package plans_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
)

func TestIntegrationAccountPlans(t *testing.T) {
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

	const owner = "00000000-0000-0000-0000-0000000000b1"
	if _, err := tx.Exec(ctx, `insert into auth.users (id, email, aud, role) values ($1, 'plan@test.mx', 'authenticated', 'authenticated')`, owner); err != nil {
		t.Fatal(err)
	}
	r := plans.Repo{DB: tx}
	if _, ok, err := r.ForUser(ctx, owner); ok || err != nil {
		t.Fatalf("account before any business: %v %v", ok, err)
	}

	// The first business starts a 14-day Pro trial; a second one doesn't restart it.
	first, err := businesses.Create(ctx, tx, owner, businesses.Input{Name: "Renta", Slug: "plan-renta", Vertical: "tool_rental"}.Normalize())
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.ForBusiness(ctx, first.ID)
	if err != nil || a.Plan != plans.Trial || a.EndsAt == nil || a.DaysLeft(time.Now()) != 14 {
		t.Fatalf("trial: %+v %v", a, err)
	}
	if err := r.Set(ctx, owner, plans.Account{Plan: plans.Lite}, "Piloto"); err != nil {
		t.Fatal(err)
	}
	second, err := businesses.Create(ctx, tx, owner, businesses.Input{Name: "Barbería", Slug: "plan-barber", Vertical: "barbershop"}.Normalize())
	if err != nil {
		t.Fatal(err)
	}
	if a, err := r.ForBusiness(ctx, second.ID); err != nil || a.Plan != plans.Lite || a.EndsAt != nil {
		t.Fatalf("second business keeps the account's plan: %+v %v", a, err)
	}
	// Both businesses copy the account's plan (what the active features read).
	var lite int
	if err := tx.QueryRow(ctx, `select count(*) from public.businesses where id in ($1, $2) and plan = 'lite'`, first.ID, second.ID).Scan(&lite); err != nil || lite != 2 {
		t.Fatalf("businesses.plan synced: %d %v", lite, err)
	}
	// Usage counts what the limits apply to: the template's active items, the businesses owned.
	var items int
	if err := tx.QueryRow(ctx, `select count(*) from public.catalog_items where business_id = $1 and active`, first.ID).Scan(&items); err != nil {
		t.Fatal(err)
	}
	if n, err := r.Usage(ctx, first.ID, "catalog_items", ""); err != nil || n != items || n == 0 {
		t.Fatalf("catalog usage %d (want %d) %v", n, items, err)
	}
	if _, err := tx.Exec(ctx, `update public.catalog_items set active = false where business_id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.Usage(ctx, first.ID, "catalog_items", ""); n != 0 {
		t.Fatalf("deactivated items still count: %d", n)
	}
	if n, err := r.OwnedBusinesses(ctx, owner); err != nil || n != 2 {
		t.Fatalf("owned: %d %v", n, err)
	}
	if o, err := r.OwnerOf(ctx, second.ID); err != nil || o != owner {
		t.Fatalf("owner: %q %v", o, err)
	}
}
