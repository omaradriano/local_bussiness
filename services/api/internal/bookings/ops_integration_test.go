package bookings

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestIntegrationF3_5_PagosYSaldo(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "nails", `{"deposit_percent": 30}`)
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6147770001"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	var inv Invalid
	if _, err := Transition(ctx, p, f.business, b.ID, "confirmed", "", ""); !errors.As(err, &inv) {
		t.Fatalf("confirm without advance: %v", err)
	}
	// Paying the advance confirms the hold by itself.
	b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "advance", Method: "transfer", Amount: 7500}, "", now, noFiles)
	if err != nil || b.Status != "confirmed" || b.HoldExpiresAt != nil || b.Balance.Due != 17500 {
		t.Fatalf("after advance: status=%s hold=%v bal=%+v err=%v", b.Status, b.HoldExpiresAt, b.Balance, err)
	}
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 20000}, "", now, noFiles); !errors.As(err, &inv) || inv.Field != "amount" {
		t.Fatalf("overpay: %v", err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "in_progress", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "completed", "", ""); !errors.As(err, &inv) {
		t.Fatalf("complete with balance due: %v", err)
	}
	b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 17500}, "", now, noFiles)
	if err != nil || b.Balance.Due != 0 || b.Balance.Paid != b.Total {
		t.Fatalf("saldo = total − pagos: %+v %v", b.Balance, err)
	}
	if b, err = Transition(ctx, p, f.business, b.ID, "completed", "", "Quedó lindo"); err != nil || b.Status != "completed" || len(b.Next) != 0 {
		t.Fatalf("complete: %+v %v", b, err)
	}
}

func TestIntegrationF3_6_TransicionesYBitacora(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "nails", `{}`) // no advance: starts confirmed
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6147770002"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now)
	if err != nil || b.Status != "confirmed" {
		t.Fatalf("create: %v %v", b.Status, err)
	}
	var inv Invalid
	for _, bad := range []string{"completed", "checked_out", "expired", "ready"} {
		if _, err := Transition(ctx, p, f.business, b.ID, bad, "", ""); !errors.As(err, &inv) {
			t.Fatalf("confirmed -> %s should be rejected: %v", bad, err)
		}
	}
	// Cancelling releases the slot: someone else can book 11:00 again.
	if b, err = Transition(ctx, p, f.business, b.ID, "cancelled", "", "Avisó por WhatsApp"); err != nil || len(b.Staff) != 0 {
		t.Fatalf("cancel: staff=%v err=%v", b.Staff, err)
	}
	if _, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6147770003"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now); err != nil {
		t.Fatalf("slot not released: %v", err)
	}
	if len(b.Events) != 2 || b.Events[1].ToStatus != "cancelled" || *b.Events[1].Note != "Avisó por WhatsApp" {
		t.Fatalf("bitácora: %+v", b.Events)
	}
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 100}, "", now, noFiles); !errors.As(err, &inv) {
		t.Fatalf("payment on a cancelled booking: %v", err)
	}
}

func TestIntegrationF3_6_PasosObligatoriosPendientes(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "nails", `{}`)
	if _, err := p.Exec(ctx, `insert into public.workflow_steps (business_id, applies_to_kind, actor, moment, to_status, step_type, label)
		values ($1, 'service', 'business', 'before_status', 'completed', 'text', 'Tonos usados')`, f.business); err != nil {
		t.Fatal(err)
	}
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6147770004"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now)
	if err != nil || len(b.Steps) != 1 || b.Steps[0].Status != "pending" {
		t.Fatalf("steps: %+v %v", b.Steps, err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "in_progress", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 25000}, "", now, noFiles); err != nil {
		t.Fatal(err)
	}
	var sp StepsPending
	if _, err := Transition(ctx, p, f.business, b.ID, "completed", "", ""); !errors.As(err, &sp) || sp.Labels[0] != "Tonos usados" {
		t.Fatalf("pending step: %v", err)
	}
	if _, err := p.Exec(ctx, `update public.booking_steps set status = 'done', completed_at = now() where booking_id = $1`, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "completed", "", ""); err != nil {
		t.Fatalf("after completing the step: %v", err)
	}
}

// F3-4: each vertical walks its whole flow (rentals: TestIntegrationRentalCheckoutAndReturn).
func TestIntegrationF3_4_FlujoCompletoPorGiro(t *testing.T) {
	p := pool(t)
	ctx := context.Background()

	t.Run("uñas: apartado → anticipo → en proceso → saldo → completada", func(t *testing.T) {
		f := setup(t, p, "nails", `{"deposit_percent": 30}`)
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6141230001"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		steps := []func() (Booking, error){
			func() (Booking, error) {
				return AddPayment(ctx, p, f.business, b.ID, Payment{Type: "advance", Method: "cash", Amount: 7500}, "", now, noFiles)
			},
			func() (Booking, error) { return Transition(ctx, p, f.business, b.ID, "in_progress", "", "") },
			func() (Booking, error) {
				return AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "card", Amount: 17500}, "", now, noFiles)
			},
			func() (Booking, error) { return Transition(ctx, p, f.business, b.ID, "completed", "", "") },
		}
		for i, s := range steps {
			if b, err = s(); err != nil {
				t.Fatalf("step %d: %v", i, err)
			}
		}
		want := []string{"pending_deposit", "confirmed", "in_progress", "completed"}
		if b.Status != "completed" || len(b.Events) != len(want) {
			t.Fatalf("status=%s events=%d", b.Status, len(b.Events))
		}
		for i, e := range b.Events {
			if e.ToStatus != want[i] {
				t.Fatalf("event %d = %s, want %s", i, e.ToStatus, want[i])
			}
		}
	})

	t.Run("flores a domicilio: confirmada → preparación → lista → en camino → completada", func(t *testing.T) {
		f := setup(t, p, "flowers", `{}`)
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6141230002"), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"),
			Options: map[string]string{"Color": "Blanco"}, Fulfillment: "delivery", DeliveryAddress: "Calle 1", Source: "admin"}, now)
		if err != nil || b.Status != "confirmed" {
			t.Fatalf("create: %v %v", b.Status, err)
		}
		for _, to := range []string{"in_progress", "ready", "out_for_delivery"} {
			if b, err = Transition(ctx, p, f.business, b.ID, to, "", ""); err != nil {
				t.Fatalf("%s: %v", to, err)
			}
		}
		AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "transfer", Amount: b.Balance.Due}, "", now, noFiles)
		if b, err = Transition(ctx, p, f.business, b.ID, "completed", "", "Entregado a la mamá"); err != nil || b.Status != "completed" {
			t.Fatalf("complete: %v %v", b.Status, err)
		}
	})

	t.Run("flores para recoger no sale a entrega", func(t *testing.T) {
		f := setup(t, p, "flowers", `{}`)
		b, _, _ := Create(ctx, p, f.business, CreateInput{Customer: cust("6141230003"), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"),
			Options: map[string]string{"Color": "Rojo"}, Source: "admin"}, now)
		Transition(ctx, p, f.business, b.ID, "in_progress", "", "")
		Transition(ctx, p, f.business, b.ID, "ready", "", "")
		var inv Invalid
		if _, err := Transition(ctx, p, f.business, b.ID, "out_for_delivery", "", ""); !errors.As(err, &inv) {
			t.Fatalf("pickup order sent out for delivery: %v", err)
		}
	})
}

func TestIntegrationReschedule(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "flowers", `{"daily_capacity": 1}`)
	order := func(phone, startsAt string) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at(startsAt),
			Options: map[string]string{"Color": "Rojo"}, Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	a := order("6147770001", "2026-11-02T18:00:00Z")
	order("6147770002", "2026-11-03T18:00:00Z")

	// Same day, another window: the order already counts there, so a full day is fine.
	b, err := Reschedule(ctx, p, f.business, a.ID, *at("2026-11-02T16:00:00Z"), at("2026-11-02T19:00:00Z"), "", now)
	if err != nil || !b.StartsAt.Equal(*at("2026-11-02T16:00:00Z")) || b.EndsAt == nil || len(b.Events) != 2 {
		t.Fatalf("same day: %+v %v", b, err)
	}
	// Nov 3 is full (capacity 1).
	if _, err := Reschedule(ctx, p, f.business, a.ID, *at("2026-11-03T16:00:00Z"), nil, "", now); !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("full day: %v", err)
	}
	// A window ending another day is refused.
	var inv Invalid
	if _, err := Reschedule(ctx, p, f.business, a.ID, *at("2026-11-04T16:00:00Z"), at("2026-11-05T16:00:00Z"), "", now); !errors.As(err, &inv) || inv.Field != "ends_at" {
		t.Fatalf("window: %v", err)
	}
	b, err = Reschedule(ctx, p, f.business, a.ID, *at("2026-11-04T16:00:00Z"), nil, "", now)
	if err != nil || !b.StartsAt.Equal(*at("2026-11-04T16:00:00Z")) || b.EndsAt != nil {
		t.Fatalf("another day: %+v %v", b, err)
	}
	// A cancelled order keeps its date.
	if _, err := Transition(ctx, p, f.business, a.ID, "cancelled", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Reschedule(ctx, p, f.business, a.ID, *at("2026-11-05T16:00:00Z"), nil, "", now); !errors.As(err, &inv) || inv.Field != "status" {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestIntegrationCancelWithPayments(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "flowers", `{"deposit_percent": 50}`)
	order := func(phone string) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"),
			Options: map[string]string{"Color": "Rojo"}, Source: "admin", InitialPayment: &Payment{Method: "transfer", Amount: 50000}}, now)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var inv Invalid
	a := order("6148880001")
	if _, err := TransitionPaid(ctx, p, f.business, a.ID, "cancelled", "", "", ""); !errors.As(err, &inv) || inv.Field != "paid" {
		t.Fatalf("cancel without choosing: %v", err)
	}
	// Given back: a payment_refund with the same method, nothing paid and nothing owed.
	got, err := TransitionPaid(ctx, p, f.business, a.ID, "cancelled", PaidRefund, "", "")
	last := got.Payments[len(got.Payments)-1]
	if err != nil || got.Balance.Paid != 0 || got.Balance.Due != 0 || last.Type != "payment_refund" || last.Amount != 50000 || last.Method != "transfer" {
		t.Fatalf("refund: %+v %+v %v", got.Balance, last, err)
	}
	// Kept as a penalty: still paid, nothing owed.
	b := order("6148880002")
	got, err = TransitionPaid(ctx, p, f.business, b.ID, "cancelled", PaidKeep, "", "Ya no lo quiso")
	if err != nil || got.Balance.Paid != 50000 || got.Balance.Due != 0 || len(got.Payments) != 1 {
		t.Fatalf("keep: %+v %v", got.Balance, err)
	}
	// Paid with two methods: each is given back the way it came in.
	c := order("6148880004")
	if _, err := AddPayment(ctx, p, f.business, c.ID, Payment{Type: "balance", Method: "cash", Amount: 7000}, "", now, noFiles); err != nil {
		t.Fatal(err)
	}
	got, err = TransitionPaid(ctx, p, f.business, c.ID, "cancelled", PaidRefund, "", "")
	refunds := map[string]int64{}
	for _, x := range got.Payments {
		if x.Type == "payment_refund" {
			refunds[x.Method] += x.Amount
		}
	}
	if err != nil || got.Balance.Paid != 0 || len(refunds) != 2 || refunds["transfer"] != 50000 || refunds["cash"] != 7000 {
		t.Fatalf("mixed refund: %+v %v %v", got.Balance, refunds, err)
	}
}

func TestIntegrationOrderExtras(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "flowers", `{}`)
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6148880003"), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"),
		Options: map[string]string{"Color": "Rojo"}, Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	var sale string
	if err := p.QueryRow(ctx, `insert into public.sales (business_id, booking_id, sold_at, method, total, cost_total) values ($1, $2, now(), 'cash', 35000, 0)
		returning id::text`, f.business, b.ID).Scan(&sale); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `insert into public.sale_lines (business_id, sale_id, name, quantity, unit_price, unit_cost) values ($1, $2, 'Globo', 1, 15000, 0), ($1, $2, 'Chocolates', 2, 10000, 0)`,
		f.business, sale); err != nil {
		t.Fatal(err)
	}
	got, err := Get(ctx, p, f.business, b.ID)
	if err != nil || len(got.Extras) != 1 || got.Extras[0].Total != 35000 || got.Extras[0].Items != "2 × Chocolates, 1 × Globo" || got.Total != b.Total {
		t.Fatalf("extras: %+v %v", got.Extras, err)
	}
}

func TestIntegrationCancelWithExtras(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "flowers", `{"deposit_percent": 50}`)
	var choc string
	if err := p.QueryRow(ctx, `insert into public.retail_products (business_id, name, price, cost, stock) values ($1, 'Chocolates', 10000, 5000, 10)
		returning id::text`, f.business).Scan(&choc); err != nil {
		t.Fatal(err)
	}
	// An order with 10,000 paid and a linked sale of 2 chocolates (already out of stock) plus a balloon.
	order := func(phone string) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"),
			Options: map[string]string{"Color": "Rojo"}, Source: "admin", InitialPayment: &Payment{Method: "cash", Amount: 10000}}, now)
		if err != nil {
			t.Fatal(err)
		}
		var sale string
		if err := p.QueryRow(ctx, `insert into public.sales (business_id, booking_id, sold_at, method, total, cost_total) values ($1, $2, now(), 'card', 35000, 10000)
			returning id::text`, f.business, b.ID).Scan(&sale); err != nil {
			t.Fatal(err)
		}
		lines := [][]any{
			{`insert into public.sale_lines (business_id, sale_id, product_id, quantity, unit_price, unit_cost) values ($1, $2, $3, 2, 10000, 5000)`, f.business, sale, choc},
			{`insert into public.sale_lines (business_id, sale_id, name, quantity, unit_price, unit_cost) values ($1, $2, 'Globo', 1, 15000, 0)`, f.business, sale},
			{`update public.retail_products set stock = stock - 2 where business_id = $1 and id = $2`, f.business, choc},
		}
		for _, l := range lines {
			if _, err := p.Exec(ctx, l[0].(string), l[1:]...); err != nil {
				t.Fatal(err)
			}
		}
		return b
	}
	stock := func() (n int) {
		p.QueryRow(ctx, `select stock from public.retail_products where id = $1`, choc).Scan(&n)
		return n
	}
	var inv Invalid
	a := order("6148880011")
	if _, err := TransitionPaid(ctx, p, f.business, a.ID, "cancelled", "", "", ""); !errors.As(err, &inv) || inv.Field != "paid" || !strings.Contains(inv.Message, "$450.00") {
		t.Fatalf("cancel without choosing should name the 450 paid with extras: %+v %v", inv, err)
	}
	// Given back: the advance is refunded and the linked sale is undone, with its stock back.
	got, err := TransitionPaid(ctx, p, f.business, a.ID, "cancelled", PaidRefund, "", "")
	if err != nil || got.Balance.Paid != 0 || len(got.Extras) != 0 || stock() != 10 {
		t.Fatalf("refund: %+v extras=%d stock=%d %v", got.Balance, len(got.Extras), stock(), err)
	}
	// Kept: the sale stays (and so does its income and stock out).
	b := order("6148880012")
	got, err = TransitionPaid(ctx, p, f.business, b.ID, "cancelled", PaidKeep, "", "")
	if err != nil || got.Balance.Paid != 10000 || len(got.Extras) != 1 || stock() != 8 {
		t.Fatalf("keep: %+v extras=%d stock=%d %v", got.Balance, len(got.Extras), stock(), err)
	}
}

// R4: extras left on the order's account are owed with it and undone if it's cancelled.
func TestIntegrationExtrasOnAccount(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "flowers", `{}`)
	var choc string
	if err := p.QueryRow(ctx, `insert into public.retail_products (business_id, name, price, cost, stock) values ($1, 'Chocolates', 13500, 5000, 8)
		returning id::text`, f.business).Scan(&choc); err != nil {
		t.Fatal(err)
	}
	order := func(phone string) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust(phone), ItemID: f.item, StartsAt: at("2026-11-02T18:00:00Z"),
			Options: map[string]string{"Color": "Rojo"}, Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		var sale string
		if err := p.QueryRow(ctx, `insert into public.sales (business_id, booking_id, sold_at, method, total, cost_total) values ($1, $2, now(), 'on_account', 27000, 10000)
			returning id::text`, f.business, b.ID).Scan(&sale); err != nil {
			t.Fatal(err)
		}
		for _, q := range [][]any{
			{`insert into public.sale_lines (business_id, sale_id, product_id, quantity, unit_price, unit_cost) values ($1, $2, $3, 2, 13500, 5000)`, f.business, sale, choc},
			{`update public.retail_products set stock = stock - 2 where business_id = $1 and id = $2`, f.business, choc},
		} {
			if _, err := p.Exec(ctx, q[0].(string), q[1:]...); err != nil {
				t.Fatal(err)
			}
		}
		return b
	}
	stock := func() (n int) {
		p.QueryRow(ctx, `select stock from public.retail_products where id = $1`, choc).Scan(&n)
		return n
	}

	// listDue: what the day's list says is left to collect from an order.
	listDue := func(id string) int64 {
		rows, err := List(ctx, p, f.business, ListFilter{From: *at("2026-11-02T00:00:00Z"), To: *at("2026-11-03T00:00:00Z")})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == id {
				return r.Due
			}
		}
		t.Fatalf("%s not listed", id)
		return 0
	}

	a := order("6148880021")
	got, err := Get(ctx, p, f.business, a.ID)
	if err != nil || got.Balance.Due != a.Total+27000 || got.Total != a.Total || listDue(a.ID) != got.Balance.Due {
		t.Fatalf("owed with extras: total %d due %d list %d %v", got.Total, got.Balance.Due, listDue(a.ID), err)
	}
	// Paying it all (order + extras) leaves nothing owed.
	got, err = AddPayment(ctx, p, f.business, a.ID, Payment{Type: "balance", Method: "cash", Amount: a.Total + 27000}, "", now, noFiles)
	if err != nil || got.Balance.Due != 0 {
		t.Fatalf("pay with extras: %+v %v", got.Balance, err)
	}

	// Cancelled with nothing paid: no choice to make, the extras on account are undone and the stock is back.
	b := order("6148880022")
	if got, err := TransitionPaid(ctx, p, f.business, b.ID, "cancelled", "", "", ""); err != nil || len(got.Extras) != 0 || stock() != 6 {
		t.Fatalf("cancel on account: extras=%d stock=%d %v", len(got.Extras), stock(), err)
	}
	if d := listDue(b.ID); d != 0 {
		t.Fatalf("a cancelled order owes nothing in the list: %d", d)
	}
}
