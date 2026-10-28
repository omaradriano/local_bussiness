package bookings

import "testing"

func TestF3_6_MachineAllowsOnlyItsTransitions(t *testing.T) {
	cases := []struct {
		kind, from, to string
		ok             bool
	}{
		// services
		{"service", "pending_deposit", "confirmed", true},
		{"service", "confirmed", "in_progress", true},
		{"service", "in_progress", "completed", true},
		{"service", "confirmed", "completed", true}, // a barbershop finishes it without starting it
		{"service", "confirmed", "no_show", true},
		{"service", "pending_deposit", "completed", false}, // can't skip the advance
		{"service", "confirmed", "checked_out", false},     // rental-only state
		{"service", "completed", "cancelled", false},       // final
		// products
		{"product", "confirmed", "in_progress", true},
		{"product", "in_progress", "ready", true},
		{"product", "ready", "out_for_delivery", true},
		{"product", "ready", "completed", true},
		{"product", "out_for_delivery", "completed", true},
		{"product", "confirmed", "no_show", false},
		// rentals
		{"rental", "confirmed", "checked_out", true},
		{"rental", "checked_out", "returned", true},
		{"rental", "overdue", "returned", true},
		{"rental", "overdue", "checked_out", true}, // only by extending it
		{"rental", "returned", "completed", true},
		{"rental", "checked_out", "completed", false}, // must be returned first
		{"rental", "checked_out", "cancelled", false}, // it's out: can't cancel
		{"rental", "confirmed", "in_progress", false},
		// everyone
		{"service", "pending_deposit", "cancelled", true},
		{"rental", "pending_deposit", "expired", true},
		{"product", "cancelled", "confirmed", false},
		{"rental", "unknown", "confirmed", false},
	}
	for _, tc := range cases {
		if got := CanTransition(tc.kind, tc.from, tc.to); got != tc.ok {
			t.Errorf("%s: %s -> %s = %v, want %v", tc.kind, tc.from, tc.to, got, tc.ok)
		}
	}
}

func TestOnlySystemMovesToExpiredAndOverdue(t *testing.T) {
	if ByPeople("expired") || ByPeople("overdue") {
		t.Fatal("expired/overdue are set by the worker, not from the panel")
	}
	if !ByPeople("cancelled") || !ByPeople("completed") {
		t.Fatal("people can cancel and complete")
	}
}

func TestReleasesResources(t *testing.T) {
	for _, s := range []string{"cancelled", "expired", "no_show"} {
		if !Releases(s) {
			t.Errorf("%s must release the time or units", s)
		}
	}
	if Releases("completed") || Releases("checked_out") {
		t.Error("completed/checked_out keep their resources (history and the rental itself)")
	}
}

func TestNextStepsForThePanel(t *testing.T) {
	got := Next("rental", "confirmed")
	want := []string{"checked_out", "cancelled", "no_show"}
	if !equalStrings(got, want) {
		t.Fatalf("Next = %v, want %v", got, want)
	}
	if got := Next("rental", "overdue"); !equalStrings(got, []string{"returned"}) {
		t.Fatalf("overdue: no check-out button (extending is its own flow), got %v", got)
	}
	if len(Next("service", "completed")) != 0 {
		t.Fatal("completed has no next steps")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStillAhead(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"confirmed", "checked_out", true},    // out-photos can be added before check-out
		{"checked_out", "checked_out", false}, // already out: they're evidence now
		{"checked_out", "completed", true},    // return photos until it's closed
		{"returned", "completed", true},
		{"completed", "completed", false},
		{"cancelled", "completed", false},
		{"overdue", "returned", true},
	}
	for _, tc := range cases {
		if got := StillAhead("rental", tc.from, tc.to); got != tc.want {
			t.Errorf("%s -> %s = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
