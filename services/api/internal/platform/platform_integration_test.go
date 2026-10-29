package platform

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
)

func TestIntegrationFeaturesPerBusiness(t *testing.T) {
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
	const rental, barber = "00000000-0000-0000-0000-0000000000f1", "00000000-0000-0000-0000-0000000000f2"
	if _, err := tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values
		($1, 'feat-renta', 'Renta', 'tool_rental'), ($2, 'feat-barberia', 'Barbería', 'barbershop')`, rental, barber); err != nil {
		t.Fatal(err)
	}
	p, cust, set := Repo{DB: tx}, customers.Repo{DB: tx}, settings.Repo{DB: tx}
	state := func(b Business, key string) features.State {
		i := slices.IndexFunc(b.Features, func(s features.State) bool { return s.Key == key })
		if i < 0 {
			t.Fatalf("%s not listed for %s", key, b.Vertical)
		}
		return b.Features[i]
	}

	// The rental giro asks for a guarantor until the platform admin removes it.
	if r, _ := cust.Rules(ctx, rental); !r.RequireGuarantor {
		t.Fatal("rental should ask for a guarantor")
	}
	off := false
	b, err := p.SetGrant(ctx, rental, "guarantor", &off)
	if err != nil || state(b, "guarantor").Available {
		t.Fatalf("remove: %+v %v", b.Features, err)
	}
	if r, _ := cust.Rules(ctx, rental); r.RequireGuarantor {
		t.Fatal("guarantor still required after removing it")
	}
	// Back to the giro.
	if b, _ = p.SetGrant(ctx, rental, "guarantor", nil); !state(b, "guarantor").Available || state(b, "guarantor").Grant != nil {
		t.Fatalf("reset: %+v", state(b, "guarantor"))
	}

	// A barbershop gets customer documents; the owner may turn them off; inventory never applies.
	on := true
	if b, err = p.SetGrant(ctx, barber, "customerDocuments", &on); err != nil || !state(b, "customerDocuments").Active {
		t.Fatalf("give: %+v %v", b.Features, err)
	}
	if _, err := p.SetGrant(ctx, barber, "inventory", &on); !errors.Is(err, ErrNotForBusiness) {
		t.Fatalf("inventory on a barbershop: %v", err)
	}
	st, _ := set.Get(ctx, barber)
	next, errs := st.Apply(settings.Patch{Features: map[string]bool{"customerDocuments": false}})
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if err := set.Save(ctx, barber, next); err != nil {
		t.Fatal(err)
	}
	if b, _ = p.Business(ctx, barber); !state(b, "customerDocuments").Available || state(b, "customerDocuments").Active {
		t.Fatalf("owner off: %+v", state(b, "customerDocuments"))
	}
	list, err := p.Businesses(ctx)
	if err != nil || len(list) < 2 {
		t.Fatalf("list: %d %v", len(list), err)
	}
}

func TestIntegrationVerticalStatus(t *testing.T) {
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
	p := Repo{DB: tx}

	// Seeded by the migration: rentals and barbershops public, nails and flowers being tried out.
	for vertical, want := range map[string]string{"tool_rental": Public, "barbershop": Public, "nails": AdminOnly, "flowers": AdminOnly} {
		if got, err := p.VerticalStatus(ctx, vertical); err != nil || got != want {
			t.Fatalf("%s: %q %v", vertical, got, err)
		}
	}
	if _, err := p.SetVerticalStatus(ctx, "nails", Public); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.VerticalStatus(ctx, "nails"); got != Public {
		t.Fatalf("after publishing: %q", got)
	}
	if _, err := p.SetVerticalStatus(ctx, "nails", "maybe"); !errors.Is(err, ErrBadStatus) {
		t.Fatalf("bad status: %v", err)
	}
	if _, err := p.VerticalStatus(ctx, "limos"); !errors.Is(err, ErrUnknownVertical) {
		t.Fatalf("unknown: %v", err)
	}
}
