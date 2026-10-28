package bookings

import (
	"testing"
	"time"
)

func TestRentalState(t *testing.T) {
	for status, want := range map[string]string{"checked_out": "rented", "overdue": "rented", "confirmed": "scheduled", "pending_deposit": "scheduled"} {
		if got := rentalState(status); got != want {
			t.Errorf("%s = %s, want %s", status, got, want)
		}
	}
}

func TestSortInventoryByDate(t *testing.T) {
	d := func(day int) *time.Time { v := time.Date(2026, 11, day, 15, 0, 0, 0, time.UTC); return &v }
	rows := []InventoryRow{
		{State: "inactive", ItemName: "Roto", UnitCode: "RM-03"},
		{State: "available", ItemName: "Roto", UnitCode: "RM-02"},
		{State: "scheduled", Date: d(9), ItemName: "Roto", UnitCode: "RM-01"},
		{State: "available", ItemName: "Andamio", UnitCode: "AN-01"},
		{State: "rented", Date: d(4), ItemName: "Roto", UnitCode: "RM-01"},
	}
	SortInventory(rows)
	want := []string{"rented RM-01", "scheduled RM-01", "available AN-01", "available RM-02", "inactive RM-03"}
	for i, r := range rows {
		if got := r.State + " " + r.UnitCode; got != want[i] {
			t.Fatalf("row %d = %s, want %s (all: %+v)", i, got, want[i], rows)
		}
	}
}
