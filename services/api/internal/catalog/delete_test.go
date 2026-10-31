package catalog

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationDeleteItemsAndUnits(t *testing.T) {
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

	const biz = "00000000-0000-0000-0000-0000000000d1"
	_, err = tx.Exec(ctx, `insert into public.businesses (id, slug, name, vertical) values ($1, 'delete-test', 'D', 'tool_rental')`, biz)
	must(err)
	r := Repo{DB: tx}
	unit, dep, fee := "day", int64(0), int64(0)
	item, err := r.Create(ctx, biz, Item{Kind: "rental", Name: "Rotomartillo", Price: 30000, PriceUnit: &unit, SecurityDeposit: &dep, LateFeePerDay: &fee})
	must(err)
	u1, err := r.CreateUnit(ctx, biz, Unit{ItemID: item.ID, Code: "RM-01", Status: "available"})
	must(err)
	u2, err := r.CreateUnit(ctx, biz, Unit{ItemID: item.ID, Code: "RM-02", Status: "available"})
	must(err)

	// RM-01 has an upcoming rental.
	var cust, booking string
	must(tx.QueryRow(ctx, `insert into public.customers (business_id, name, phone) values ($1, 'Ana', '+526140000001') returning id::text`, biz).Scan(&cust))
	must(tx.QueryRow(ctx, `insert into public.bookings (business_id, customer_id, kind, status, starts_at, ends_at, source)
		values ($1, $2, 'rental', 'confirmed', now() + interval '1 day', now() + interval '2 days', 'admin') returning id::text`, biz, cust).Scan(&booking))
	_, err = tx.Exec(ctx, `insert into public.booking_items (business_id, booking_id, item_id, unit_price) values ($1, $2, $3, 30000)`, biz, booking, item.ID)
	must(err)
	_, err = tx.Exec(ctx, `insert into public.booking_resources (business_id, booking_id, item_unit_id, during)
		values ($1, $2, $3, tstzrange(now() + interval '1 day', now() + interval '2 days'))`, biz, booking, u1.ID)
	must(err)

	if err := r.DeleteUnit(ctx, biz, item.ID, u1.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("unit with an upcoming rental: %v", err)
	}
	if err := r.Delete(ctx, biz, item.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("item with an upcoming rental: %v", err)
	}

	// A free unit goes away and its code can be used again.
	must(r.DeleteUnit(ctx, biz, item.ID, u2.ID))
	if units, _ := r.ListUnits(ctx, biz, item.ID); len(units) != 1 || units[0].Code != "RM-01" {
		t.Fatalf("units after delete: %+v", units)
	}
	if _, err := r.GetUnit(ctx, biz, item.ID, u2.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted unit still found: %v", err)
	}
	if _, err := r.CreateUnit(ctx, biz, Unit{ItemID: item.ID, Code: "RM-02", Status: "available"}); err != nil {
		t.Fatalf("reuse the code: %v", err)
	}

	// Once the rental is over, the item can go (with its units); the booking keeps it.
	_, err = tx.Exec(ctx, `update public.bookings set status = 'completed' where id = $1`, booking)
	must(err)
	must(r.Delete(ctx, biz, item.ID))
	if items, _ := r.List(ctx, biz); len(items) != 0 {
		t.Fatalf("deleted item listed: %+v", items)
	}
	if _, err := r.Get(ctx, biz, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted item found: %v", err)
	}
	if err := r.Delete(ctx, biz, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	var left int
	must(tx.QueryRow(ctx, `select count(*) from public.item_units where item_id = $1 and deleted_at is null`, item.ID).Scan(&left))
	if left != 0 {
		t.Fatalf("units left: %d", left)
	}
}
