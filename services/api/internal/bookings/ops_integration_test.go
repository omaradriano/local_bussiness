package bookings

import (
	"context"
	"errors"
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
	b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "advance", Method: "transfer", Amount: 7500}, "", now)
	if err != nil || b.Status != "confirmed" || b.HoldExpiresAt != nil || b.Balance.Due != 17500 {
		t.Fatalf("after advance: status=%s hold=%v bal=%+v err=%v", b.Status, b.HoldExpiresAt, b.Balance, err)
	}
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 20000}, "", now); !errors.As(err, &inv) || inv.Field != "amount" {
		t.Fatalf("overpay: %v", err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "in_progress", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "completed", "", ""); !errors.As(err, &inv) {
		t.Fatalf("complete with balance due: %v", err)
	}
	b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 17500}, "", now)
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
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 100}, "", now); !errors.As(err, &inv) {
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
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: 25000}, "", now); err != nil {
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
				return AddPayment(ctx, p, f.business, b.ID, Payment{Type: "advance", Method: "cash", Amount: 7500}, "", now)
			},
			func() (Booking, error) { return Transition(ctx, p, f.business, b.ID, "in_progress", "", "") },
			func() (Booking, error) {
				return AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "card", Amount: 17500}, "", now)
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
		AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "transfer", Amount: b.Balance.Due}, "", now)
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
