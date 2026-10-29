package finances

import (
	"slices"
	"testing"
)

func days(ds ...string) []Day {
	out := make([]Day, len(ds))
	for i, s := range ds {
		out[i] = d(s)
	}
	return out
}

func TestRecurringDueDates(t *testing.T) {
	cases := []struct {
		name string
		r    Recurring
		from string
		to   string
		want []Day
	}{
		{"mensual el 31 se recorre al fin de mes", Recurring{Frequency: "monthly", StartsOn: d("2026-01-31")},
			"2026-01-01", "2026-04-30", days("2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30")},
		{"quincenal: el día y 15 después", Recurring{Frequency: "biweekly", StartsOn: d("2026-11-01")},
			"2026-11-01", "2026-12-31", days("2026-11-01", "2026-11-16", "2026-12-01", "2026-12-16")},
		{"semanal dentro del periodo", Recurring{Frequency: "weekly", StartsOn: d("2026-10-05")},
			"2026-11-01", "2026-11-15", days("2026-11-02", "2026-11-09")},
		{"anual", Recurring{Frequency: "yearly", StartsOn: d("2024-02-29")},
			"2025-01-01", "2026-12-31", days("2025-02-28", "2026-02-28")},
		{"hasta: nada después", Recurring{Frequency: "monthly", StartsOn: d("2026-01-10"), EndsOn: ptr(d("2026-02-10"))},
			"2026-01-01", "2026-12-31", days("2026-01-10", "2026-02-10")},
		{"antes de empezar: nada", Recurring{Frequency: "monthly", StartsOn: d("2027-01-01")},
			"2026-01-01", "2026-12-31", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.r.DueIn(d(c.from), d(c.to)); !slices.EqualFunc(got, c.want, func(a, b Day) bool { return a.Equal(b) }) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestRecurringPendingAndNext(t *testing.T) {
	rent := Recurring{Frequency: "monthly", StartsOn: d("2026-08-01")}
	// Aug paid, Sep skipped; Oct and Nov due by 5 Nov and not answered.
	answered := map[string]bool{"2026-08-01": true, "2026-09-01": true}
	got := rent.Pending(d("2026-11-05"), answered)
	if !slices.EqualFunc(got, days("2026-10-01", "2026-11-01"), func(a, b Day) bool { return a.Equal(b) }) {
		t.Fatalf("pending: %v", got)
	}
	if n := rent.NextDue(d("2026-11-05")); n == nil || !n.Equal(d("2026-12-01")) {
		t.Fatalf("next: %v", n)
	}
	ended := Recurring{Frequency: "monthly", StartsOn: d("2026-01-01"), EndsOn: ptr(d("2026-03-01"))}
	if n := ended.NextDue(d("2026-05-01")); n != nil {
		t.Fatalf("next after end: %v", n)
	}
	// A very old one only asks for the last year of charges.
	old := Recurring{Frequency: "weekly", StartsOn: d("2020-01-01")}
	if p := old.Pending(d("2026-11-05"), nil); len(p) == 0 || p[0].Before(d("2025-11-05")) {
		t.Fatalf("old pending starts %v (%d)", p[0], len(p))
	}
	if !rent.IsDue(d("2026-10-01")) || rent.IsDue(d("2026-10-02")) || rent.IsDue(d("2026-07-01")) {
		t.Fatal("IsDue")
	}
}

func TestRecurringValidate(t *testing.T) {
	ok := Recurring{Description: "Renta del local", Category: "rent", Amount: 800000, Frequency: "monthly", StartsOn: d("2026-11-01")}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]Recurring{
		"description": {Description: " ", Category: "rent", Amount: 1, Frequency: "monthly", StartsOn: d("2026-11-01")},
		"amount":      {Description: "x", Category: "rent", Amount: 0, Frequency: "monthly", StartsOn: d("2026-11-01")},
		"frequency":   {Description: "x", Category: "rent", Amount: 1, Frequency: "daily", StartsOn: d("2026-11-01")},
		"ends_on":     {Description: "x", Category: "rent", Amount: 1, Frequency: "monthly", StartsOn: d("2026-11-01"), EndsOn: ptr(d("2026-10-01"))},
	}
	for field, r := range bad {
		if err, _ := r.Validate().(Invalid); err[field] == "" {
			t.Errorf("%s: want error, got %v", field, err)
		}
	}
}

func ptr[T any](v T) *T { return &v }
