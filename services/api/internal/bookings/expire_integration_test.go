package bookings

import (
	"context"
	"testing"
	"time"
)

func TestIntegrationF3_3_ApartadoSinAnticipoExpiraYLibera(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "nails", `{"deposit_percent": 30, "hold_hours": 4}`)
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6149990001"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now)
	if err != nil || b.Status != "pending_deposit" {
		t.Fatalf("hold: %v %v", b.Status, err)
	}
	// Before the deadline nothing happens.
	ExpireHolds(ctx, p, now.Add(3*time.Hour))
	if got, _ := Get(ctx, p, f.business, b.ID); got.Status != "pending_deposit" {
		t.Fatalf("expired too early: %s", got.Status)
	}
	// After it, the hold expires and the slot is free again.
	if _, err := ExpireHolds(ctx, p, now.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ := Get(ctx, p, f.business, b.ID)
	if got.Status != "expired" || len(got.Staff) != 0 || got.Events[len(got.Events)-1].ToStatus != "expired" {
		t.Fatalf("after deadline: status=%s staff=%v", got.Status, got.Staff)
	}
	if _, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6149990002"), ItemID: f.item, StartsAt: at("2026-11-02T17:00:00Z"), Source: "admin"}, now.Add(5*time.Hour)); err != nil {
		t.Fatalf("slot not released: %v", err)
	}
	// Paid holds never expire (they were confirmed).
	paid, _, _ := Create(ctx, p, f.business, CreateInput{Customer: cust("6149990003"), ItemID: f.item, StartsAt: at("2026-11-02T19:00:00Z"), Source: "admin"}, now)
	AddPayment(ctx, p, f.business, paid.ID, Payment{Type: "advance", Method: "cash", Amount: 7500}, "", now)
	ExpireHolds(ctx, p, now.Add(48*time.Hour))
	if got, _ := Get(ctx, p, f.business, paid.ID); got.Status != "confirmed" {
		t.Fatalf("paid hold expired: %s", got.Status)
	}
}

func TestIntegrationRentasVencidas(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	f := setup(t, p, "tool_rental", `{}`)
	p.Exec(ctx, `delete from public.workflow_steps where business_id = $1`, f.business)
	b, _, err := Create(ctx, p, f.business, CreateInput{Customer: cust("6149990004"), ItemID: f.item,
		StartsAt: at("2026-11-02T15:00:00Z"), EndsAt: at("2026-11-04T15:00:00Z"), Source: "admin"}, now)
	if err != nil {
		t.Fatal(err)
	}
	AddPayment(ctx, p, f.business, b.ID, Payment{Type: "security_deposit", Method: "cash", Amount: 100000}, "", now)
	if _, err := Checkout(ctx, p, f.business, b.ID, "", "", now); err != nil {
		t.Fatal(err)
	}
	due := *at("2026-11-04T15:00:00Z")
	MarkOverdue(ctx, p, due.Add(30*time.Minute)) // within grace
	if got, _ := Get(ctx, p, f.business, b.ID); got.Status != "checked_out" {
		t.Fatalf("overdue within grace: %s", got.Status)
	}
	MarkOverdue(ctx, p, due.Add(2*time.Hour))
	got, _ := Get(ctx, p, f.business, b.ID)
	if got.Status != "overdue" {
		t.Fatalf("not overdue: %s", got.Status)
	}
	// It can still be returned.
	if got, err = Return(ctx, p, f.business, b.ID, "", "", due.Add(26*time.Hour)); err != nil || got.Status != "returned" || got.Rental.LateDays != 1 {
		t.Fatalf("return overdue: %v %+v %v", got.Status, got.Rental, err)
	}
}
