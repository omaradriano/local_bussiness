package catalog

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationCatalogCRUDIsScopedToBusiness(t *testing.T) {
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

	const bizA, bizB = "00000000-0000-0000-0000-00000000000a", "00000000-0000-0000-0000-00000000000b"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'cat-a', 'A', 'tool_rental'), ($2, 'cat-b', 'B', 'nails')`, bizA, bizB); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}

	unit, dep, fee := "day", int64(100000), int64(30000)
	created, err := r.Create(ctx, bizA, Item{Kind: "rental", Name: "Rotomartillo", Price: 30000, PriceUnit: &unit, SecurityDeposit: &dep, LateFeePerDay: &fee})
	if err != nil || created.ID == "" || !created.Active || created.Sort != 1 {
		t.Fatalf("create: %+v %v", created, err)
	}

	updated := created.Apply(Patch{Price: ptr(int64(35000)), Active: ptr(false)})
	if got, err := r.Update(ctx, bizA, updated); err != nil || got.Price != 35000 || got.Active {
		t.Fatalf("update: %+v %v", got, err)
	}

	if _, err := r.Get(ctx, bizB, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get from other business: %v, want ErrNotFound", err)
	}
	if _, err := r.Update(ctx, bizB, updated); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update from other business: %v, want ErrNotFound", err)
	}
	if items, err := r.List(ctx, bizB); err != nil || len(items) != 0 {
		t.Fatalf("list other business: %v %v", items, err)
	}
	if items, err := r.List(ctx, bizA); err != nil || len(items) != 1 {
		t.Fatalf("list own business: %v %v", items, err)
	}
}
