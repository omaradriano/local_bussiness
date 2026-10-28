package businesses

import (
	"context"
	"errors"
	"os"
	"strings"
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
		// Any giro, present or future: the slug comes from its key ("tool_rental" -> "prueba-tool-rental").
		in := Input{Name: "Prueba " + v, Slug: "prueba-" + strings.ReplaceAll(v, "_", "-"), Vertical: v}.Normalize()
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
		// Single-person nails business: the owner attends every service with the business hours.
		var ownerStaff, ownerServices, ownerHours int
		err = outer.QueryRow(ctx, `
			select count(m.staff_id),
			       (select count(*) from public.staff_services ss join public.business_members m2 on m2.staff_id = ss.staff_id where m2.business_id = $1 and m2.user_id = $2),
			       (select count(*) from public.business_hours h join public.business_members m3 on m3.staff_id = h.staff_id where m3.business_id = $1 and m3.user_id = $2)
			from public.business_members m where m.business_id = $1 and m.user_id = $2`, b.ID, user).Scan(&ownerStaff, &ownerServices, &ownerHours)
		if err != nil {
			t.Fatal(err)
		}
		if tpl.OwnerAttends && (ownerStaff != 1 || ownerServices != len(tpl.Items) || ownerHours == 0) {
			t.Fatalf("%s: owner as staff: staff=%d services=%d hours=%d", v, ownerStaff, ownerServices, ownerHours)
		}
		if !tpl.OwnerAttends && ownerStaff != 0 {
			t.Fatalf("%s: owner should not be staff", v)
		}
		var units, want int
		outer.QueryRow(ctx, `select count(*) from public.item_units where business_id = $1`, b.ID).Scan(&units)
		for _, it := range tpl.Items {
			want += it.Units
		}
		if units != want {
			t.Fatalf("%s: units = %d, want %d", v, units, want)
		}
	}

	_, err = Create(ctx, outer, user, Input{Name: "Otra", Slug: "prueba-nails", Vertical: "nails"}.Normalize())
	if !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("duplicate slug: got %v, want ErrSlugTaken", err)
	}
}
