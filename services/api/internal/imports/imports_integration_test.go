package imports

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
)

func TestIntegrationImports(t *testing.T) {
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
	const biz = "00000000-0000-0000-0000-0000000000f7"
	for _, q := range []string{
		`insert into public.businesses (id, slug, name, vertical, plan) values ('` + biz + `', 'import-a', 'A', 'barbershop', 'pro')`,
		`insert into public.customers (business_id, name, phone) values ('` + biz + `', 'Ana', '+526141234567')`,
		`insert into public.catalog_items (business_id, kind, name, price, duration_minutes) values ('` + biz + `', 'service', 'Corte', 15000, 30)`,
		`insert into public.retail_products (business_id, name, price, cost, stock) values ('` + biz + `', 'Cera', 15000, 7000, 3)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	r := Repo{DB: tx}
	count := func(q string) int {
		var n int
		if err := tx.QueryRow(ctx, q, biz).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Preview changes nothing; applying creates one and updates the other.
	rows := []customers.Input{{Name: s("Ana López"), Phone: s("614 123 4567")}, {Name: s("Luis"), Phone: s("6149998877")}, {Name: s("")}}
	if sum, err := r.Customers(ctx, biz, rows, now, false); err != nil || sum.Created != 1 || sum.Updated != 1 || sum.Failed != 1 || sum.Applied {
		t.Fatalf("preview: %+v %v", sum, err)
	}
	if n := count(`select count(*) from public.customers where business_id = $1`); n != 1 {
		t.Fatalf("preview wrote: %d customers", n)
	}
	if sum, err := r.Customers(ctx, biz, rows, now, true); err != nil || !sum.Applied {
		t.Fatalf("apply: %+v %v", sum, err)
	}
	if n := count(`select count(*) from public.customers where business_id = $1 and name in ('Ana López', 'Luis')`); n != 2 {
		t.Fatalf("customers after import: %d", n)
	}

	if sum, err := r.Catalog(ctx, biz, []CatalogRow{{Name: "CORTE", Price: i64(18000)}, {Name: "Barba", Price: i64(10000), DurationMinutes: n(20)}}, true); err != nil || sum.Created != 1 || sum.Updated != 1 {
		t.Fatalf("catalog: %+v %v", sum, err)
	}
	if n := count(`select count(*) from public.catalog_items where business_id = $1 and ((name = 'Corte' and price = 18000) or (name = 'Barba' and kind = 'service'))`); n != 2 {
		t.Fatalf("catalog after import: %d", n)
	}

	if sum, err := r.Products(ctx, biz, []products.ProductInput{{Name: s("cera"), Price: i64(16000), Stock: n(99)}, {Name: s("Gel"), Price: i64(9000), Cost: i64(4000), Stock: n(5)}}, true); err != nil || sum.Created != 1 || sum.Updated != 1 {
		t.Fatalf("products: %+v %v", sum, err)
	}
	if n := count(`select count(*) from public.retail_products where business_id = $1 and ((name = 'Cera' and price = 16000 and stock = 3) or (name = 'Gel' and stock = 5))`); n != 2 {
		t.Fatalf("products after import (existing stock kept): %d", n)
	}
}
