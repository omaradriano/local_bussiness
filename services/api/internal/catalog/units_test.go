package catalog

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestUnitApplyAndValidate(t *testing.T) {
	u := Unit{Code: " rm-01 ", Status: "available"}.Apply(UnitPatch{})
	if u.Code != "RM-01" || len(u.Validate()) != 0 {
		t.Fatalf("got %+v %v", u, u.Validate())
	}
	if errs := (Unit{Code: "", Status: "available"}).Validate(); errs["code"] == "" {
		t.Error("empty code accepted")
	}
	if errs := (Unit{Code: strings.Repeat("X", 41), Status: "available"}).Validate(); errs["code"] == "" {
		t.Error("long code accepted")
	}
	if errs := (Unit{Code: "RM-01", Status: "lost"}).Validate(); errs["status"] == "" {
		t.Error("unknown status accepted")
	}
	m := u.Apply(UnitPatch{Status: ptr("maintenance"), Notes: ptr(" cambiar carbones ")})
	if m.Status != "maintenance" || m.Notes != "cambiar carbones" || m.Code != "RM-01" {
		t.Fatalf("patch: %+v", m)
	}
}

func TestIntegrationUnitsOnlyForRentalsOfTheBusiness(t *testing.T) {
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
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'units-a', 'A', 'tool_rental'), ($2, 'units-b', 'B', 'nails')`, bizA, bizB); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	unit, dep, fee, dur := "day", int64(1), int64(1), 60
	rental, _ := r.Create(ctx, bizA, Item{Kind: "rental", Name: "Andamio", Price: 15000, PriceUnit: &unit, SecurityDeposit: &dep, LateFeePerDay: &fee})
	service, _ := r.Create(ctx, bizB, Item{Kind: "service", Name: "Gelish", Price: 25000, DurationMinutes: &dur})

	u, err := r.CreateUnit(ctx, bizA, Unit{ItemID: rental.ID, Code: "AN-01", Status: "available"})
	if err != nil || u.ID == "" {
		t.Fatalf("create: %+v %v", u, err)
	}
	// A unique violation aborts the transaction, so try it inside a savepoint.
	sp, _ := tx.Begin(ctx)
	_, err = Repo{DB: sp}.CreateUnit(ctx, bizA, Unit{ItemID: rental.ID, Code: "AN-01", Status: "available"})
	sp.Rollback(ctx)
	if !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("duplicate code: %v", err)
	}
	if _, err := r.CreateUnit(ctx, bizB, Unit{ItemID: rental.ID, Code: "AN-02", Status: "available"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other business: %v", err)
	}
	if _, err := r.CreateUnit(ctx, bizB, Unit{ItemID: service.ID, Code: "X-1", Status: "available"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("service item: %v", err)
	}
	if _, err := r.ListUnits(ctx, bizB, service.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("list units of a service: %v", err)
	}
	u.Status = "maintenance"
	if got, err := r.UpdateUnit(ctx, bizA, u); err != nil || got.Status != "maintenance" {
		t.Fatalf("update: %+v %v", got, err)
	}
	if units, err := r.ListUnits(ctx, bizA, rental.ID); err != nil || len(units) != 1 {
		t.Fatalf("list: %v %v", units, err)
	}
}
