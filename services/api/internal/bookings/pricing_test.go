package bookings

import (
	"testing"
	"time"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct{ in, want string }{
		{"614 123 4567", "+526141234567"},
		{"(614) 123-4567", "+526141234567"},
		{"+52 614 123 4567", "+526141234567"},
		{"526141234567", "+526141234567"},
		{"+1 915 555 0100", "+19155550100"},
		{"12345", ""},
		{"", ""},
		{"abc", ""},
	}
	for _, tc := range cases {
		if got := NormalizePhone(tc.in); got != tc.want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRentalPeriods(t *testing.T) {
	h := time.Hour
	cases := []struct {
		unit string
		dur  time.Duration
		want int64
	}{
		{"day", 24 * h, 1},
		{"day", 25 * h, 2}, // a started day counts
		{"day", 3 * h, 1},
		{"day", 72 * h, 3},
		{"weekend", 48 * h, 1},
		{"weekend", 72 * h, 1},
		{"weekend", 73 * h, 2},
		{"week", 24 * h, 1},
		{"week", 7 * 24 * h, 1},
		{"week", 8 * 24 * h, 2},
	}
	for _, tc := range cases {
		if got := RentalPeriods(tc.unit, tc.dur); got != tc.want {
			t.Errorf("RentalPeriods(%s, %v) = %d, want %d", tc.unit, tc.dur, got, tc.want)
		}
	}
}

func TestQuote(t *testing.T) {
	cases := []struct {
		name string
		in   QuoteInput
		want Quote
	}{
		{"servicio con 30% de anticipo", QuoteInput{UnitPrice: 25000, Quantity: 1, Periods: 1, DepositPercent: 30},
			Quote{Subtotal: 25000, Total: 25000, Advance: 7500}},
		{"producto con envío; el anticipo incluye el envío", QuoteInput{UnitPrice: 65000, Quantity: 2, Periods: 1, DeliveryFee: 9000, DepositPercent: 50},
			Quote{Subtotal: 130000, Total: 139000, Advance: 69500}},
		{"renta de 3 días x 2 unidades, sin anticipo, con depósito de garantía", QuoteInput{UnitPrice: 30000, Quantity: 2, Periods: 3, SecurityDeposit: 100000},
			Quote{Subtotal: 180000, Total: 180000, Advance: 0, SecurityDeposit: 200000}},
		{"redondeo del anticipo a centavos enteros", QuoteInput{UnitPrice: 33333, Quantity: 1, Periods: 1, DepositPercent: 30},
			Quote{Subtotal: 33333, Total: 33333, Advance: 10000}},
		{"anticipo fuera de rango se acota a 100%", QuoteInput{UnitPrice: 1000, Quantity: 1, Periods: 1, DepositPercent: 150},
			Quote{Subtotal: 1000, Total: 1000, Advance: 1000}},
		{"opciones con costo extra por unidad", QuoteInput{UnitPrice: 45000, OptionsExtra: 5000, Quantity: 2, Periods: 1},
			Quote{Subtotal: 100000, Total: 100000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compute(tc.in); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestInitialStatus(t *testing.T) {
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	st, hold := InitialStatus(7500, 4, now)
	if st != "pending_deposit" || hold == nil || !hold.Equal(now.Add(4*time.Hour)) {
		t.Fatalf("with advance: %s %v", st, hold)
	}
	if st, hold := InitialStatus(0, 4, now); st != "confirmed" || hold != nil {
		t.Fatalf("without advance: %s %v", st, hold)
	}
	if _, hold := InitialStatus(100, 0, now); hold == nil || !hold.Equal(now.Add(DefaultHoldHours*time.Hour)) {
		t.Fatalf("default hold: %v", hold)
	}
}

// With «el día de regreso cuenta completo» a rental charges calendar days in the business's zone.
func TestRentalPeriodsByCalendarDay(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chihuahua")
	at := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, loc) }
	cases := []struct {
		name       string
		unit       string
		start, end time.Time
		want       int64
	}{
		{"same day", "day", at(2, 9), at(2, 18), 1},
		{"next day, same hour", "day", at(2, 14), at(3, 14), 1},
		{"next day, late at night: no extra day", "day", at(2, 14), at(3, 22), 1},
		{"next day 11:59 pm", "day", at(2, 14), time.Date(2026, 11, 3, 23, 59, 0, 0, loc), 1},
		{"two days later", "day", at(2, 14), at(4, 9), 2},
		{"weekend: up to 3 days", "weekend", at(6, 10), at(9, 20), 1},
		{"weekend: a 4th day", "weekend", at(6, 10), at(10, 9), 2},
		{"week", "week", at(2, 10), at(9, 22), 1},
	}
	for _, tc := range cases {
		if got := RentalPeriodsByDay(tc.unit, CalendarDays(loc, tc.start, tc.end)); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}
