package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/steps"
)

func TestIntegrationRentalCheckoutAndReturn(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{}`) // fixture: "Foto del INE" (photo) required before checked_out
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6148880001"), ItemID: f.item,
		StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-04T15:00:00Z"), Source: "admin"}, now)
	if err != nil || b.Status != "confirmed" {
		t.Fatalf("create: %v %v", b.Status, err)
	}
	var sp StepsPending
	if _, err := Checkout(ctx, p, f.business, b.ID, "", "", now); !errors.As(err, &sp) || sp.Labels[0] != "Foto del INE" {
		t.Fatalf("checkout without ID photo: %v", err)
	}
	var inv Invalid
	if _, err := Transition(ctx, p, f.business, b.ID, "checked_out", "", ""); !errors.As(err, &inv) {
		t.Fatalf("generic transition to checked_out must be refused: %v", err)
	}

	// Complete the photo step: only keys under the booking's prefix, and they must exist.
	stepID := b.Steps[0].ID
	if bucket, err := StepUpload(ctx, p, f.business, b.ID, stepID); err != nil || bucket != "rental-evidence" {
		t.Fatalf("step upload bucket: %q %v", bucket, err)
	}
	ok := func(context.Context, string, string) error { return nil }
	missing := func(context.Context, string, string) error { return errors.New("no object") }
	key := NewFileKey(f.business, b.ID, "jpg")
	if _, err := CompleteStep(ctx, p, f.business, b.ID, stepID, steps.Submission{Files: []string{"otro/negocio/x.jpg"}}, "", now, ok); !errors.As(err, &inv) || inv.Field != "files" {
		t.Fatalf("foreign key: %v", err)
	}
	if _, err := CompleteStep(ctx, p, f.business, b.ID, stepID, steps.Submission{Files: []string{key}}, "", now, missing); !errors.As(err, &inv) {
		t.Fatalf("missing object: %v", err)
	}
	if _, err := CompleteStep(ctx, p, f.business, b.ID, stepID, steps.Submission{}, "", now, ok); !errors.As(err, &inv) || inv.Field != "data" {
		t.Fatalf("no photos: %v", err)
	}
	if b, err = CompleteStep(ctx, p, f.business, b.ID, stepID, steps.Submission{Files: []string{key}, Data: json.RawMessage(`{}`)}, "", now, ok); err != nil || b.Steps[0].Status != "done" {
		t.Fatalf("complete step: %+v %v", b.Steps, err)
	}

	// Security deposit (1000 pesos in the fixture) must be held before the units go out.
	if _, err := Checkout(ctx, p, f.business, b.ID, "", "", now); !errors.As(err, &inv) {
		t.Fatalf("checkout without deposit: %v", err)
	}
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "security_deposit", Method: "cash", Amount: 100000}, "", now); err != nil {
		t.Fatal(err)
	}
	if b, err = Checkout(ctx, p, f.business, b.ID, "Sin golpes, con broca", "", now); err != nil || b.Status != "checked_out" || b.Rental.CheckedOutAt == nil {
		t.Fatalf("checkout: %+v %v", b.Rental, err)
	}
	var outPhotos []string
	p.QueryRow(ctx, `select out_photos from public.rental_checkouts where booking_id = $1`, b.ID).Scan(&outPhotos)
	if len(outPhotos) != 1 || outPhotos[0] != key {
		t.Fatalf("evidence on the checkout record: %v", outPhotos)
	}

	// Early return (Nov 3, a day before due): the unit is free from then on.
	early := time.Date(2026, 11, 3, 16, 0, 0, 0, time.UTC)
	if b, err = Return(ctx, p, f.business, b.ID, "Regresó limpio", "", early); err != nil || b.Status != "returned" || b.Rental.LateDays != 0 {
		t.Fatalf("return: %+v %v", b.Rental, err)
	}
	if _, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6148880002"), ItemID: f.item,
		StartsAt: at("2026-11-03T18:00:00Z"), EndsAt: at("2026-11-04T18:00:00Z"), Source: "admin"}, early); err != nil {
		t.Fatalf("unit not released by the early return: %v", err)
	}

	// Refund the deposit and close it.
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: b.Balance.Due}, "", now); err != nil {
		t.Fatal(err)
	}
	if b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "refund", Method: "cash", Amount: 100000}, "", now); err != nil || b.Balance.DepositHeld != 0 {
		t.Fatalf("refund: %+v %v", b.Balance, err)
	}
	if b, err = Transition(ctx, p, f.business, b.ID, "completed", "", ""); err != nil || b.Status != "completed" {
		t.Fatalf("complete: %v %v", b.Status, err)
	}
}

func TestIntegrationLateReturnSuggestsFee(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{}`)
	if _, err := p.Exec(ctx, `delete from public.workflow_steps where business_id = $1`, f.business); err != nil {
		t.Fatal(err)
	}
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6148880003"), ItemID: f.item,
		StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-04T15:00:00Z"), Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	AddPayment(ctx, p, f.business, b.ID, Payment{Type: "security_deposit", Method: "cash", Amount: 100000}, "", now)
	if _, err := Checkout(ctx, p, f.business, b.ID, "", "", now); err != nil {
		t.Fatal(err)
	}
	late := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC) // ~43 h late -> 2 days
	b, err = Return(ctx, p, f.business, b.ID, "", "", late)
	if err != nil || b.Rental.LateDays != 2 || b.Rental.LateFeeSuggested != 2*30000 {
		t.Fatalf("late return: %+v %v", b.Rental, err)
	}
}

func TestIntegrationCustomerDocsAreReused(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"security_deposit_enabled": false}`)
	if _, err := p.Exec(ctx, `delete from public.workflow_steps where business_id = $1`, f.business); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `insert into public.workflow_steps (business_id, applies_to_kind, actor, moment, to_status, step_type, label, config, sort) values
		  ($1, 'rental', 'business', 'before_status', 'checked_out', 'photo', 'Foto del INE', '{"bucket":"private-docs","customer_doc":"ine"}', 1),
		  ($1, 'rental', 'business', 'before_status', 'checked_out', 'photo', 'Comprobante de domicilio', '{"bucket":"private-docs","customer_doc":"proof_of_address"}', 2)`, f.business); err != nil {
		t.Fatal(err)
	}
	rent := func(start, end string) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030001"), ItemID: f.item, StartsAt: at(start), EndsAt: at(end), Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	ok := func(context.Context, string, string) error { return nil }

	first := rent("2026-11-02T15:00:00Z", "2026-11-03T15:00:00Z")
	var sp StepsPending
	if _, err := Checkout(ctx, p, f.business, first.ID, "", "", now); !errors.As(err, &sp) || len(sp.Labels) != 2 {
		t.Fatalf("new customer, nothing on file: %v", err)
	}
	// The ID uploaded on this rental goes to the customer's profile too.
	if _, err := CompleteStep(ctx, p, f.business, first.ID, first.Steps[0].ID, steps.Submission{Files: []string{NewFileKey(f.business, first.ID, "jpg")}}, "", now, ok); err != nil {
		t.Fatal(err)
	}

	second := rent("2026-11-05T15:00:00Z", "2026-11-06T15:00:00Z")
	if !second.Steps[0].OnFile || second.Steps[1].OnFile {
		t.Fatalf("on file flags: %+v", second.Steps)
	}
	if _, err := Checkout(ctx, p, f.business, second.ID, "", "", now); !errors.As(err, &sp) || len(sp.Labels) != 1 || sp.Labels[0] != "Comprobante de domicilio" {
		t.Fatalf("ID on file, proof missing: %v", err)
	}
	// A trusted customer doesn't need documents.
	if _, err := p.Exec(ctx, `update public.customers set trusted = true where business_id = $1`, f.business); err != nil {
		t.Fatal(err)
	}
	b, err := Checkout(ctx, p, f.business, second.ID, "", "", now)
	if err != nil || b.Status != "checked_out" {
		t.Fatalf("trusted customer checkout: %v %v", b.Status, err)
	}
	if b.Steps[0].Status != "done" || len(b.Steps[0].Files) != 1 {
		t.Fatalf("ID step filled from the profile: %+v", b.Steps[0])
	}
}

func TestIntegrationInitialPayment(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"deposit_percent": 50}`)
	create := func(day int, pay *Payment) (Booking, error) { // Tue 3 … Fri 6; 1 day = 30000
		start := fmt.Sprintf("2026-11-%02dT15:00:00Z", day)
		end := fmt.Sprintf("2026-11-%02dT14:00:00Z", day+1)
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030002"), ItemID: f.item, StartsAt: at(start), EndsAt: at(end), Source: "admin", InitialPayment: pay}, now)
		return b, err
	}
	if b, err := create(3, nil); err != nil || b.Status != "pending_deposit" || len(b.Payments) != 0 {
		t.Fatalf("no payment: %v %v", b.Status, err)
	}
	b, err := create(4, &Payment{Method: "cash", Amount: 15000})
	if err != nil || b.Status != "confirmed" || b.Balance.Due != 15000 || len(b.Payments) != 1 || b.Payments[0].Type != "advance" {
		t.Fatalf("half up front: %+v %+v %v", b.Status, b.Balance, err)
	}
	b, err = create(5, &Payment{Method: "transfer", Amount: 30000})
	if err != nil || b.Balance.Due != 0 || b.Payments[0].Type != "balance" {
		t.Fatalf("all up front: %+v %v", b.Balance, err)
	}
	var bad Invalid
	if _, err := create(6, &Payment{Method: "cash", Amount: 40000}); !errors.As(err, &bad) || bad.Field != "amount" {
		t.Fatalf("more than the total: %v", err)
	}
	if _, err := create(6, &Payment{Method: "cheque", Amount: 100}); !errors.As(err, &bad) || bad.Field != "method" {
		t.Fatalf("bad method: %v", err)
	}
	if b, err := create(6, &Payment{Method: "card"}); err != nil || b.Balance.Due != 0 || b.Payments[0].Amount != 30000 {
		t.Fatalf("no amount = the whole total: %+v %v", b.Balance, err)
	}
}

func TestIntegrationExtendRental(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{}`)
	rent := func(start, end string) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030003"), ItemID: f.item, StartsAt: at(start), EndsAt: at(end), Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first := rent("2026-11-02T15:00:00Z", "2026-11-03T15:00:00Z") // Mon → Tue, 1 day
	rent("2026-11-06T15:00:00Z", "2026-11-07T15:00:00Z")          // Fri → Sat, the only unit

	b, err := Extend(ctx, p, f.business, first.ID, *at("2026-11-05T15:00:00Z"), "", now)
	if err != nil || b.Total != 90000 || !b.EndsAt.Equal(*at("2026-11-05T15:00:00Z")) {
		t.Fatalf("extend to Thu: total %d %v", b.Total, err)
	}
	if _, err := Extend(ctx, p, f.business, first.ID, *at("2026-11-07T15:00:00Z"), "", now); !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("over the next rental: %v", err)
	}
	var bad Invalid
	if _, err := Extend(ctx, p, f.business, first.ID, *at("2026-11-04T15:00:00Z"), "", now); !errors.As(err, &bad) || bad.Field != "ends_at" {
		t.Fatalf("earlier date: %v", err)
	}
	// An overdue rental extended goes back to "rented".
	if _, err := p.Exec(ctx, `update public.bookings set status = 'overdue' where id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	b, err = Extend(ctx, p, f.business, first.ID, *at("2026-11-06T14:00:00Z"), "", now)
	if err != nil || b.Status != "checked_out" {
		t.Fatalf("overdue extended: %v %v", b.Status, err)
	}
	// Sunday the business is closed: no return that day.
	other := rent("2026-11-09T15:00:00Z", "2026-11-10T15:00:00Z")
	if _, err := Extend(ctx, p, f.business, other.ID, *at("2026-11-15T18:00:00Z"), "", now); !errors.As(err, &bad) || bad.Field != "ends_at" {
		t.Fatalf("return on Sunday: %v", err)
	}
}
