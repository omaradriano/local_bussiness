package customers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func s(v string) *string { return &v }

var rental = Rules{RequireAddress: true, RequireGuarantor: true}

func TestValidateRentalCustomer(t *testing.T) {
	full := Customer{}.Apply(Input{Name: s(" Ana López "), Phone: s("614 123 4567"), Address: s("Calle 1 #2, Chihuahua"),
		GuarantorName: s("Pedro López"), GuarantorPhone: s("614 765 4321")})
	if errs := full.Validate(rental); len(errs) != 0 {
		t.Fatalf("complete rental customer: %v", errs)
	}
	if full.Name != "Ana López" || full.Phone != "+526141234567" || *full.GuarantorPhone != "+526147654321" {
		t.Fatalf("normalized: %+v", full)
	}

	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"sin dirección", Input{Address: s("  ")}, "address"},
		{"sin aval", Input{GuarantorName: s("")}, "guarantor_name"},
		{"sin teléfono del aval", Input{GuarantorPhone: s("")}, "guarantor_phone"},
		{"teléfono del aval inválido", Input{GuarantorPhone: s("123")}, "guarantor_phone"},
		{"teléfono inválido", Input{Phone: s("614 123")}, "phone"},
		{"sin nombre", Input{Name: s("")}, "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if errs := full.Apply(tc.in).Validate(rental); errs[tc.field] == "" {
				t.Fatalf("want error on %s, got %v", tc.field, errs)
			}
		})
	}
	// Other verticals only need name and phone.
	if errs := (Customer{}.Apply(Input{Name: s("Ana"), Phone: s("6141234567")})).Validate(Rules{}); len(errs) != 0 {
		t.Fatalf("nails customer: %v", errs)
	}
}

func TestIntegrationCustomerProfiles(t *testing.T) {
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
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'cus-a', 'A', 'tool_rental'), ($2, 'cus-b', 'B', 'tool_rental')`, bizA, bizB); err != nil {
		t.Fatal(err)
	}
	r := Repo{DB: tx}
	c := Customer{}.Apply(Input{Name: s("Ana López"), Phone: s("6141234567"), Address: s("Calle 1"), GuarantorName: s("Pedro"), GuarantorPhone: s("6147654321")})
	ana, err := r.Create(ctx, bizA, c)
	if err != nil || ana.ID == "" {
		t.Fatalf("create: %v", err)
	}
	// Same phone in the same business: refused (in a savepoint: a unique violation aborts the tx).
	sp, _ := tx.Begin(ctx)
	_, err = Repo{DB: sp}.Create(ctx, bizA, c)
	sp.Rollback(ctx)
	if !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("duplicate phone: %v", err)
	}
	if id, _ := r.IDByPhone(ctx, bizA, "+526141234567"); id != ana.ID {
		t.Fatalf("IDByPhone = %s", id)
	}
	// Search by name fragment and by phone digits; other businesses don't see her.
	for _, q := range []string{"lóp", "4567", ""} {
		if got, err := r.Search(ctx, bizA, q, 20); err != nil || len(got) != 1 {
			t.Fatalf("search %q: %v %v", q, got, err)
		}
	}
	if got, _ := r.Search(ctx, bizB, "Ana", 20); len(got) != 0 {
		t.Fatalf("other business sees her: %v", got)
	}
	key := NewFileKey(bizA, ana.ID, "jpg")
	got, err := r.AddDocuments(ctx, bizA, ana.ID, "ine", []string{key}, "")
	if err != nil || !got.HasDoc("ine") || got.HasDoc("proof_of_address") || !strings.HasPrefix(got.Documents[0].Key, FilePrefix(bizA, ana.ID)) {
		t.Fatalf("documents: %+v %v", got.Documents, err)
	}
	trusted := true
	if up, err := r.Update(ctx, bizA, got.Apply(Input{Trusted: &trusted})); err != nil || !up.Trusted || len(up.Documents) != 1 {
		t.Fatalf("update: %+v %v", up, err)
	}
	if _, err := r.Get(ctx, bizB, ana.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get from other business: %v", err)
	}
}
