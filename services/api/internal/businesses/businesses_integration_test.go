package businesses

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationCreateAppliesTemplate(t *testing.T) {
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
	outer, err := conn.Begin(ctx) // Create's own tx becomes a savepoint; everything is rolled back
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Rollback(ctx)

	const user = "00000000-0000-0000-0000-0000000000a1"
	if _, err := outer.Exec(ctx, `insert into auth.users (id, email, aud, role) values ($1, 'o@test.mx', 'authenticated', 'authenticated')`, user); err != nil {
		t.Fatal(err)
	}

	for _, v := range Verticals {
		in := Input{Name: "Prueba " + v, Slug: "prueba-" + map[string]string{"nails": "unas", "flowers": "flores", "tool_rental": "renta"}[v], Vertical: v}.Normalize()
		b, err := Create(ctx, outer, user, in)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		var role string
		var items, hours, steps int
		err = outer.QueryRow(ctx, `
			select (select role from public.business_members where business_id = $1 and user_id = $2),
			       (select count(*) from public.catalog_items where business_id = $1),
			       (select count(*) from public.business_hours where business_id = $1),
			       (select count(*) from public.workflow_steps where business_id = $1)`, b.ID, user).Scan(&role, &items, &hours, &steps)
		if err != nil {
			t.Fatal(err)
		}
		tpl, _ := TemplateFor(v)
		if role != "owner" || items != len(tpl.Items) || hours == 0 || steps != len(tpl.Steps) {
			t.Fatalf("%s: role=%s items=%d hours=%d steps=%d", v, role, items, hours, steps)
		}
	}

	_, err = Create(ctx, outer, user, Input{Name: "Otra", Slug: "prueba-unas", Vertical: "nails"}.Normalize())
	if !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("duplicate slug: got %v, want ErrSlugTaken", err)
	}
}
