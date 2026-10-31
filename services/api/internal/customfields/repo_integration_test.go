package customfields

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
)

func TestIntegrationFieldsAndValues(t *testing.T) {
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const biz, other = "00000000-0000-0000-0000-0000000000c1", "00000000-0000-0000-0000-0000000000c2"
	_, err = tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'cf-a', 'A', 'tool_rental'), ($2, 'cf-b', 'B', 'tool_rental')`, biz, other)
	must(err)

	r := Repo{DB: tx}
	brand, err := r.Create(ctx, biz, Field{Kind: "rental", Target: "item", Label: "Marca", Type: "text", Required: true})
	must(err)
	cond, err := r.Create(ctx, biz, Field{Kind: "rental", Target: "unit", Label: "Condición", Type: "select", Options: []string{"Nueva", "Buena"}})
	must(err)
	if !brand.Active || cond.Sort <= brand.Sort || len(cond.Options) != 2 {
		t.Fatalf("created: %+v %+v", brand, cond)
	}
	if list, _ := r.List(ctx, biz, "rental"); len(list) != 2 || list[0].ID != brand.ID {
		t.Fatalf("list: %+v", list)
	}
	if list, _ := r.List(ctx, other, ""); len(list) != 0 {
		t.Fatalf("another business sees them: %+v", list)
	}
	if _, err := r.Get(ctx, other, brand.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get from another business: %v", err)
	}
	cond.Options = []string{"Nueva", "Buena", "Regular"}
	cond, err = r.Update(ctx, biz, cond)
	must(err)
	if len(cond.Options) != 3 {
		t.Fatalf("update: %+v", cond)
	}

	// Values are stored with the item and the unit.
	fields, _ := r.List(ctx, biz, "rental")
	itemValues, errs := Clean(fields, "item", map[string]any{brand.ID: "DeWalt"}, nil)
	unitValues, errs2 := Clean(fields, "unit", map[string]any{cond.ID: "Buena"}, nil)
	if len(errs)+len(errs2) != 0 {
		t.Fatalf("clean: %v %v", errs, errs2)
	}
	c := catalog.Repo{DB: tx}
	unit, dep, fee := "day", int64(0), int64(0)
	it, err := c.Create(ctx, biz, catalog.Item{Kind: "rental", Name: "Rotomartillo", Price: 30000, PriceUnit: &unit, SecurityDeposit: &dep, LateFeePerDay: &fee,
		CustomValues: itemValues})
	must(err)
	u, err := c.CreateUnit(ctx, biz, catalog.Unit{ItemID: it.ID, Code: "RM-01", Status: "available", CustomValues: unitValues})
	must(err)
	if it.CustomValues[brand.ID] != "DeWalt" || u.CustomValues[cond.ID] != "Buena" {
		t.Fatalf("values: %v %v", it.CustomValues, u.CustomValues)
	}
	// Without values the column stays an empty object, never null.
	plain, err := c.Create(ctx, biz, catalog.Item{Kind: "rental", Name: "Andamio", Price: 15000, PriceUnit: &unit, SecurityDeposit: &dep, LateFeePerDay: &fee})
	must(err)
	if plain.CustomValues == nil || len(plain.CustomValues) != 0 {
		t.Fatalf("empty values: %v", plain.CustomValues)
	}

	must(r.Delete(ctx, biz, brand.ID))
	if err := r.Delete(ctx, biz, brand.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}
