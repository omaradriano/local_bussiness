package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
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
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "security_deposit", Method: "cash", Amount: 100000}, "", now, noFiles); err != nil {
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
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "cash", Amount: b.Balance.Due}, "", now, noFiles); err != nil {
		t.Fatal(err)
	}
	if b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "refund", Method: "cash", Amount: 100000}, "", now, noFiles); err != nil || b.Balance.DepositHeld != 0 {
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
	AddPayment(ctx, p, f.business, b.ID, Payment{Type: "security_deposit", Method: "cash", Amount: 100000}, "", now, noFiles)
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

func TestIntegrationInventory(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"security_deposit_enabled": false}`)
	var spare, broken string
	if err := p.QueryRow(ctx, `insert into public.item_units (business_id, item_id, code) values ($1, $2, 'RM-02') returning id::text`, f.business, f.item).Scan(&spare); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `insert into public.item_units (business_id, item_id, code, status) values ($1, $2, 'RM-03', 'maintenance') returning id::text`, f.business, f.item).Scan(&broken); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `delete from public.workflow_steps where business_id = $1`, f.business); err != nil {
		t.Fatal(err)
	}
	rent := func(start, end string, qty int) Booking {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030004"), ItemID: f.item, Quantity: qty, StartsAt: at(start), EndsAt: at(end), Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	out := rent("2026-11-02T15:00:00Z", "2026-11-04T15:00:00Z", 1)
	if _, err := Checkout(ctx, p, f.business, out.ID, "", "", now); err != nil {
		t.Fatal(err)
	}
	rent("2026-11-09T15:00:00Z", "2026-11-10T15:00:00Z", 2) // both active units, later
	cancelled := rent("2026-11-12T15:00:00Z", "2026-11-13T15:00:00Z", 1)
	if _, err := Transition(ctx, p, f.business, cancelled.ID, "cancelled", "", ""); err != nil {
		t.Fatal(err)
	}

	// A unit whose only rental is already finished still shows up as available.
	var done string
	if err := p.QueryRow(ctx, `insert into public.item_units (business_id, item_id, code) values ($1, $2, 'RM-04') returning id::text`, f.business, f.item).Scan(&done); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `insert into public.booking_resources (business_id, booking_id, item_unit_id, during)
		values ($1, $2, $3, tstzrange('2026-10-01', '2026-10-02'))`, f.business, cancelled.ID, done); err != nil {
		t.Fatal(err)
	}
	rows, err := Inventory(ctx, p, f.business)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.State+" "+r.UnitCode)
	}
	// RM-01 is out (returns the 4th) and booked again the 9th with RM-02; RM-02 is free now; RM-03 in maintenance.
	want := []string{"rented RM-01", "scheduled RM-01", "scheduled RM-02", "available RM-02", "available RM-04", "inactive RM-03"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if !rows[0].Date.Equal(*at("2026-11-04T15:00:00Z")) || *rows[0].CustomerName != "Ana" {
		t.Fatalf("rented row: %+v", rows[0])
	}
}

func TestIntegrationQuoteMatchesCreate(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"charge_closed_days": false}`)
	// Sat → Mon over a closed Sunday that isn't charged: 1 day, plus the 1000-peso deposit.
	q, err := QuoteBooking(ctx, p, f.business, QuoteRequest{ItemID: f.item, Quantity: 1, StartsAt: *at("2026-11-07T15:00:00Z"), EndsAt: at("2026-11-09T15:00:00Z")})
	if err != nil || q.Total != 30000 || q.Periods != 1 || q.SecurityDeposit != 100000 {
		t.Fatalf("quote: %+v %v", q, err)
	}
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030005"), ItemID: f.item, StartsAt: at("2026-11-07T15:00:00Z"), EndsAt: at("2026-11-09T15:00:00Z"), Source: "admin"}, now)
	if err != nil || b.Total != q.Total {
		t.Fatalf("create total %d vs quote %d: %v", b.Total, q.Total, err)
	}
	var bad Invalid
	if _, err := QuoteBooking(ctx, p, f.business, QuoteRequest{ItemID: f.item, StartsAt: *at("2026-11-07T15:00:00Z"), EndsAt: at("2026-11-08T15:00:00Z")}); !errors.As(err, &bad) {
		t.Fatalf("return on Sunday: %v", err)
	}
}

func TestIntegrationMorePhotosAndPaymentEvidence(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"security_deposit_enabled": false}`)
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030006"), ItemID: f.item, StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-03T15:00:00Z"), Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	ok := func(context.Context, string, string) error { return nil }
	step := b.Steps[0].ID
	key := func() string { return NewFileKey(f.business, b.ID, "jpg") }
	if _, err := CompleteStep(ctx, p, f.business, b.ID, step, steps.Submission{Files: []string{key()}}, "", now, ok); err != nil {
		t.Fatal(err)
	}
	// Before check-out the done photo step takes more photos.
	b, err = CompleteStep(ctx, p, f.business, b.ID, step, steps.Submission{Files: []string{key()}}, "", now, ok)
	if err != nil || len(b.Steps[0].Files) != 2 || b.Steps[0].Status != "done" {
		t.Fatalf("append: %+v %v", b.Steps[0], err)
	}
	// Removing a photo while the step can change: 2 → 1, then 1 → 0 makes it pending again; then put one back.
	b, err = RemoveStepFile(ctx, p, f.business, b.ID, step, b.Steps[0].Files[0])
	if err != nil || len(b.Steps[0].Files) != 1 || b.Steps[0].Status != "done" {
		t.Fatalf("remove one: %+v %v", b.Steps[0], err)
	}
	b, err = RemoveStepFile(ctx, p, f.business, b.ID, step, b.Steps[0].Files[0])
	if err != nil || len(b.Steps[0].Files) != 0 || b.Steps[0].Status != "pending" {
		t.Fatalf("remove last: %+v %v", b.Steps[0], err)
	}
	if _, err := RemoveStepFile(ctx, p, f.business, b.ID, step, "otra/llave.jpg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown key: %v", err)
	}
	if b, err = CompleteStep(ctx, p, f.business, b.ID, step, steps.Submission{Files: []string{key()}}, "", now, ok); err != nil {
		t.Fatal(err)
	}
	// Late fee before the tool comes back: refused.
	var bad Invalid
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "late_fee", Method: "cash", Amount: 100}, "", now, ok); !errors.As(err, &bad) || bad.Field != "type" {
		t.Fatalf("late fee before return: %v", err)
	}
	// Transfer with its evidence photo; a key from elsewhere is refused.
	if _, err := AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "transfer", Amount: 100, Evidence: []string{key()}}, "", now, ok); !errors.As(err, &bad) || bad.Field != "evidence" {
		t.Fatalf("evidence outside payments/: %v", err)
	}
	ev := PaymentFilePrefix(f.business, b.ID) + RandomName("jpg")
	b, err = AddPayment(ctx, p, f.business, b.ID, Payment{Type: "balance", Method: "transfer", Amount: 10000, Evidence: []string{ev}}, "", now, ok)
	if err != nil || len(b.Payments) != 1 || len(b.Payments[0].Evidence) != 1 || b.Payments[0].Evidence[0] != ev {
		t.Fatalf("payment evidence: %+v %v", b.Payments, err)
	}
	// After check-out the out-photos are evidence: no more changes.
	if _, err := Checkout(ctx, p, f.business, b.ID, "", "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteStep(ctx, p, f.business, b.ID, step, steps.Submission{Files: []string{key()}}, "", now, ok); !errors.As(err, &bad) || bad.Field != "step" {
		t.Fatalf("append after check-out: %v", err)
	}
	out, _ := Get(ctx, p, f.business, b.ID)
	if _, err := RemoveStepFile(ctx, p, f.business, b.ID, step, out.Steps[0].Files[0]); !errors.As(err, &bad) || bad.Field != "step" {
		t.Fatalf("remove after check-out: %v", err)
	}
}

func TestIntegrationHistory(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{}`)
	var ids []string
	for i, day := range []string{"02", "03", "04"} {
		b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust(fmt.Sprintf("614303001%d", i)), ItemID: f.item,
			StartsAt: at("2026-11-" + day + "T15:00:00Z"), EndsAt: at("2026-11-" + day + "T20:00:00Z"), Source: "admin"}, now)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, b.ID)
	}
	if _, err := Transition(ctx, p, f.business, ids[0], "cancelled", "", ""); err != nil {
		t.Fatal(err)
	}
	all, err := History(ctx, p, f.business, HistoryFilter{Limit: 10})
	if err != nil || len(all) != 3 || all[0].ID != ids[2] || all[2].ID != ids[0] {
		t.Fatalf("newest first: %v %v", all, err)
	}
	if got, _ := History(ctx, p, f.business, HistoryFilter{Group: "cancelled", Limit: 10}); len(got) != 1 || got[0].ID != ids[0] {
		t.Fatalf("cancelled: %v", got)
	}
	// Paging: 2 then the rest, without repeats.
	page1, _ := History(ctx, p, f.business, HistoryFilter{Limit: 2})
	last := page1[len(page1)-1]
	page2, _ := History(ctx, p, f.business, HistoryFilter{Limit: 2, Before: &last.StartsAt, BeforeID: last.ID})
	if len(page1) != 2 || len(page2) != 1 || page2[0].ID != ids[0] {
		t.Fatalf("paging: %v / %v", page1, page2)
	}
	// Search by phone digits and by item name; one customer's rentals.
	if got, _ := History(ctx, p, f.business, HistoryFilter{Query: "6143030011", Limit: 10}); len(got) != 1 || got[0].ID != ids[1] {
		t.Fatalf("by phone: %v", got)
	}
	if got, _ := History(ctx, p, f.business, HistoryFilter{Query: "roto", Limit: 10}); len(got) != 3 {
		t.Fatalf("by item: %v", got)
	}
	var cid string
	if err := p.QueryRow(ctx, `select customer_id::text from public.bookings where id = $1`, ids[2]).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	if got, _ := History(ctx, p, f.business, HistoryFilter{CustomerID: cid, Limit: 10}); len(got) != 1 || got[0].ID != ids[2] {
		t.Fatalf("by customer: %v", got)
	}
}

func TestIntegrationAdditionalDocuments(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"security_deposit_enabled": false}`)
	repo := settings.Repo{DB: p}
	pagare, err := repo.CreateDocumentType(ctx, f.business, settings.DocumentType{Name: "Pagaré", Required: true, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateDocumentType(ctx, f.business, settings.DocumentType{Name: "Contrato", Active: false}); err != nil {
		t.Fatal(err)
	}
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030020"), ItemID: f.item, StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-03T15:00:00Z"), Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	// Only the active configured type is copied to the new rental, and it blocks the check-out.
	var labels []string
	for _, st := range b.Steps {
		labels = append(labels, st.Label)
	}
	if !slices.Contains(labels, "Pagaré") || slices.Contains(labels, "Contrato") {
		t.Fatalf("steps: %v", labels)
	}
	var sp StepsPending
	if _, err := Checkout(ctx, p, f.business, b.ID, "", "", now); !errors.As(err, &sp) || !slices.Contains(sp.Labels, "Pagaré") {
		t.Fatalf("checkout without the note: %v", err)
	}
	// Turning it off affects new rentals only.
	off := false
	if _, err := repo.UpdateDocumentType(ctx, f.business, pagare.Apply(settings.DocumentTypeInput{Active: &off})); err != nil {
		t.Fatal(err)
	}
	if list, _ := repo.DocumentTypes(ctx, f.business); len(list) != 2 || list[0].Active {
		t.Fatalf("types: %+v", list)
	}

	// Free documents: any name, keys under documents/, removable until the rental is closed.
	ok := func(context.Context, string, string) error { return nil }
	key := DocumentFilePrefix(f.business, b.ID) + RandomName("pdf")
	var bad Invalid
	if _, err := AddDocument(ctx, p, f.business, b.ID, "Contrato", []string{NewFileKey(f.business, b.ID, "pdf")}, "", ok); !errors.As(err, &bad) || bad.Field != "files" {
		t.Fatalf("key outside documents/: %v", err)
	}
	b, err = AddDocument(ctx, p, f.business, b.ID, "  Contrato   firmado ", []string{key}, "", ok)
	if err != nil || len(b.Documents) != 1 || b.Documents[0].Name != "Contrato firmado" {
		t.Fatalf("add: %+v %v", b.Documents, err)
	}
	b, keys, err := RemoveDocument(ctx, p, f.business, b.ID, b.Documents[0].ID)
	if err != nil || len(b.Documents) != 0 || len(keys) != 1 || keys[0] != key {
		t.Fatalf("remove: %+v %v %v", b.Documents, keys, err)
	}
	if _, err := Transition(ctx, p, f.business, b.ID, "cancelled", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := AddDocument(ctx, p, f.business, b.ID, "Tarde", []string{key}, "", ok); !errors.As(err, &bad) || bad.Field != "status" {
		t.Fatalf("add after cancel: %v", err)
	}
}

func TestIntegrationGeneralSettingsDriveBookings(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{"late_fee_multiplier": 2}`)
	repo := settings.Repo{DB: p}
	st, err := repo.Get(ctx, f.business)
	if err != nil {
		t.Fatal(err)
	}
	next, errs := st.Apply(settings.Patch{General: &settings.GeneralPatch{DepositPercent: ptrInt(50), HoldHours: ptrInt(6)}})
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if err := repo.Save(ctx, f.business, next); err != nil {
		t.Fatal(err)
	}
	again, _ := repo.Get(ctx, f.business)
	if again.General.DepositPercent != 50 || again.Rentals.LateFeeMultiplier != 2 {
		t.Fatalf("saved sections lost a key: %+v", again)
	}
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6143030030"), ItemID: f.item, StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-03T15:00:00Z"), Source: "admin"}, now)
	if err != nil || b.Status != "pending_deposit" || b.DepositRequired != 15000 || !b.HoldExpiresAt.Equal(now.Add(6*time.Hour)) {
		t.Fatalf("advance from Ajustes: %v %d %v %v", b.Status, b.DepositRequired, b.HoldExpiresAt, err)
	}
}

func ptrInt(v int) *int { return &v }
