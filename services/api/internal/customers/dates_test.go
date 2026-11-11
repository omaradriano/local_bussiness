package customers

import (
	"testing"
	"time"
)

func TestNextOccurrence(t *testing.T) {
	today := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		month, day int
		want       string
		days       int
	}{
		{11, 11, "2026-11-11", 0},  // today
		{11, 18, "2026-11-18", 7},  // this year, a week ahead
		{3, 14, "2027-03-14", 123}, // already passed: next year
		{2, 29, "2027-02-28", 109}, // not a leap year: the 28th
	}
	for _, c := range cases {
		got, days := NextOccurrence(c.month, c.day, today)
		if got.Format(time.DateOnly) != c.want || days != c.days {
			t.Errorf("%d/%d: got %s in %d days, want %s in %d", c.day, c.month, got.Format(time.DateOnly), days, c.want, c.days)
		}
	}
	if got, _ := NextOccurrence(2, 29, time.Date(2028, 1, 10, 0, 0, 0, 0, time.UTC)); got.Format(time.DateOnly) != "2028-02-29" {
		t.Errorf("leap year keeps the 29th: %s", got)
	}
}

func TestValidateDate(t *testing.T) {
	if err := (Date{Label: "Cumpleaños de su esposa", Month: 2, Day: 29}).Validate(); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for name, d := range map[string]Date{
		"sin nombre":   {Label: " ", Month: 3, Day: 14},
		"31 de abril":  {Label: "x", Month: 4, Day: 31},
		"mes 13":       {Label: "x", Month: 13, Day: 1},
		"nombre largo": {Label: string(make([]rune, 81)), Month: 1, Day: 1},
	} {
		if d.Validate() == nil {
			t.Errorf("%s: should fail", name)
		}
	}
}
