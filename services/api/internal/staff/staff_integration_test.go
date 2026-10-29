package staff

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationStaffServicesAndHours(t *testing.T) {
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
	const svc, product = "00000000-0000-0000-0000-0000000000c1", "00000000-0000-0000-0000-0000000000c2"
	for _, q := range []string{
		`insert into public.businesses (id, slug, name, vertical) values ('` + bizA + `', 'staff-a', 'A', 'nails'), ('` + bizB + `', 'staff-b', 'B', 'nails')`,
		`insert into public.catalog_items (id, business_id, kind, name, price, duration_minutes) values ('` + svc + `', '` + bizA + `', 'service', 'Gelish', 25000, 60)`,
		`insert into public.catalog_items (id, business_id, kind, name, price, prep_hours) values ('` + product + `', '` + bizA + `', 'product', 'Kit', 1000, 0)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	r := Repo{DB: tx}

	if _, err := tx.Exec(ctx, `insert into public.business_hours (business_id, weekday, opens_at, closes_at) values ($1, 2, '10:00', '19:00')`, bizA); err != nil {
		t.Fatal(err)
	}
	m, err := r.Create(ctx, bizA, Member{Name: "Karla", Color: "#E91E63"})
	// A new person can take appointments right away: every active service and the business hours.
	if err != nil || !m.Active || len(m.ServiceIDs) != 1 || m.ServiceIDs[0] != svc || len(m.Hours) != 1 || m.Hours[0].OpensAt != "10:00" {
		t.Fatalf("create: %+v %v", m, err)
	}
	if err := r.SetServices(ctx, bizA, m.ID, []string{svc, svc}); err != nil {
		t.Fatalf("set services: %v", err)
	}
	if err := r.SetServices(ctx, bizA, m.ID, []string{product}); !errors.Is(err, ErrNotAService) {
		t.Fatalf("product as service: %v", err)
	}
	if err := r.SetServices(ctx, bizB, m.ID, []string{svc}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other business staff: %v", err)
	}
	split := []Hours{{1, "09:00", "14:00"}, {1, "16:00", "20:00"}}
	if err := r.SetHours(ctx, bizA, m.ID, split); err != nil {
		t.Fatalf("set hours: %v", err)
	}
	if err := r.SetHours(ctx, bizA, "", []Hours{{1, "09:00", "20:00"}}); err != nil {
		t.Fatalf("set business hours: %v", err)
	}

	got, err := r.Get(ctx, bizA, m.ID)
	if err != nil || len(got.ServiceIDs) != 1 || len(got.Hours) != 2 || got.Hours[1].OpensAt != "16:00" {
		t.Fatalf("get: %+v %v", got, err)
	}
	if bh, err := r.BusinessHours(ctx, bizA); err != nil || len(bh) != 1 || bh[0].ClosesAt != "20:00" {
		t.Fatalf("business hours: %+v %v", bh, err)
	}
	// Replacing staff hours must not touch the general hours, and vice versa.
	if err := r.SetHours(ctx, bizA, m.ID, nil); err != nil {
		t.Fatal(err)
	}
	if bh, _ := r.BusinessHours(ctx, bizA); len(bh) != 1 {
		t.Fatalf("general hours touched: %+v", bh)
	}
	got.Active = false
	if err := r.Update(ctx, bizB, got); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update other business: %v", err)
	}

	// Deleting a person: gone from the list and the agenda (services, hours), not from other businesses.
	if err := r.Delete(ctx, bizB, m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete from another business: %v", err)
	}
	if err := r.Delete(ctx, bizA, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, bizA, m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted person still listed: %v", err)
	}
	var services, hours int
	if err := tx.QueryRow(ctx, `select (select count(*) from public.staff_services where staff_id = $1), (select count(*) from public.business_hours where staff_id = $1)`, m.ID).Scan(&services, &hours); err != nil || services+hours != 0 {
		t.Fatalf("deleted person keeps services %d / hours %d (%v)", services, hours, err)
	}
	if err := r.Delete(ctx, bizA, m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted twice: %v", err)
	}
	// The person linked to the owner can't be deleted.
	if _, err := tx.Exec(ctx, `insert into auth.users (id, email, aud, role) values ('00000000-0000-0000-0000-0000000000d1', 'staff-owner@test.mx', 'authenticated', 'authenticated')`); err != nil {
		t.Fatal(err)
	}
	owner, err := r.Create(ctx, bizA, Member{Name: "Dueña", Color: "#000000"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `insert into public.business_members (business_id, user_id, role, staff_id) values ($1, '00000000-0000-0000-0000-0000000000d1', 'owner', $2)`, bizA, owner.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, bizA, owner.ID); !errors.Is(err, ErrOwnerStaff) {
		t.Fatalf("deleted the owner: %v", err)
	}
}
