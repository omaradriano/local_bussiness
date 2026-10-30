// Package bookings creates and operates bookings (appointments, orders and rentals).
// Money is always integer centavos (MXN).
package bookings

import (
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/phone"
)

// NormalizePhone returns E.164 or "" (see phone.Normalize).
func NormalizePhone(in string) string { return phone.Normalize(in) }

// RentalPeriods: how many price units a rental lasts. A started unit counts as a whole one.
// day = 24 h, weekend = up to 3 days (Fri–Mon), week = 7 days.
func RentalPeriods(unit string, d time.Duration) int64 {
	size := map[string]time.Duration{"day": 24 * time.Hour, "weekend": 72 * time.Hour, "week": 7 * 24 * time.Hour}[unit]
	if size == 0 || d <= 0 {
		return 1
	}
	return int64((d + size - 1) / size)
}

// CalendarDays counts the local dates from start's to end's (0 = same day), in the business's zone.
func CalendarDays(loc *time.Location, start, end time.Time) int64 {
	date := func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	return int64(date(end).Sub(date(start)) / (24 * time.Hour))
}

// RentalPeriodsByDay: price units when the return day counts whole (until 11:59 pm): a day, a
// weekend (3 days) or a week (7) per period; a same-day rental is one.
func RentalPeriodsByDay(unit string, days int64) int64 {
	size := map[string]int64{"day": 1, "weekend": 3, "week": 7}[unit]
	if size == 0 || days <= 0 {
		return 1
	}
	return (days + size - 1) / size
}

type QuoteInput struct {
	UnitPrice       int64 // item price per unit (and per period for rentals)
	OptionsExtra    int64 // extra per unit for the chosen options
	Quantity        int64
	Periods         int64 // rentals: number of price units; otherwise 1
	DeliveryFee     int64
	DepositPercent  int64 // advance the business asks for, % of total
	SecurityDeposit int64 // per unit, refundable (rentals)
}

type Quote struct {
	Subtotal        int64 `json:"subtotal"`
	Total           int64 `json:"total"`
	Advance         int64 `json:"deposit_required"`
	SecurityDeposit int64 `json:"security_deposit"`
}

func Compute(in QuoteInput) Quote {
	sub := (in.UnitPrice + in.OptionsExtra) * in.Quantity * max(in.Periods, 1)
	total := sub + in.DeliveryFee
	pct := min(max(in.DepositPercent, 0), 100)
	return Quote{
		Subtotal:        sub,
		Total:           total,
		Advance:         (total*pct + 50) / 100, // rounded to whole centavos
		SecurityDeposit: in.SecurityDeposit * in.Quantity,
	}
}

const DefaultHoldHours = 24

// InitialStatus: with an advance to pay the booking is a hold that expires; otherwise it's confirmed.
func InitialStatus(advance int64, holdHours int, now time.Time) (string, *time.Time) {
	if advance <= 0 {
		return "confirmed", nil
	}
	if holdHours <= 0 {
		holdHours = DefaultHoldHours
	}
	exp := now.Add(time.Duration(holdHours) * time.Hour)
	return "pending_deposit", &exp
}
