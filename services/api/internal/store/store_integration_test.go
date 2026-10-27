package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Runs against a real Postgres with the Supabase migrations (CI: `supabase db start`).
// Everything happens inside a transaction that is rolled back.
func withTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(ctx) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })
	return ctx, tx
}

func TestIntegrationF1_3_RolYMembresias(t *testing.T) {
	ctx, tx := withTx(t)
	const (
		user  = "00000000-0000-0000-0000-0000000000a1"
		bizA  = "00000000-0000-0000-0000-00000000000a"
		bizB  = "00000000-0000-0000-0000-00000000000b"
		other = "00000000-0000-0000-0000-0000000000b1"
	)
	for _, q := range []string{
		`insert into auth.users (id, email, aud, role) values ('` + user + `', 'a@test.mx', 'authenticated', 'authenticated'),
		   ('` + other + `', 'b@test.mx', 'authenticated', 'authenticated')`,
		`insert into public.businesses (id, slug, name, vertical) values ('` + bizA + `', 'test-a', 'A', 'nails'), ('` + bizB + `', 'test-b', 'B', 'flowers')`,
		`insert into public.business_members (business_id, user_id, role) values ('` + bizA + `', '` + user + `', 'owner'), ('` + bizB + `', '` + other + `', 'owner')`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s := New(tx)

	if role, err := s.Role(ctx, bizA, user); err != nil || role != "owner" {
		t.Fatalf("Role(A) = %q, %v; want owner", role, err)
	}
	if role, err := s.Role(ctx, bizB, user); err != nil || role != "" {
		t.Fatalf("Role(B) = %q, %v; want empty (not a member)", role, err)
	}
	ms, err := s.Memberships(ctx, user)
	if err != nil || len(ms) != 1 || ms[0].Slug != "test-a" || ms[0].Role != "owner" {
		t.Fatalf("Memberships = %+v, %v", ms, err)
	}
}
