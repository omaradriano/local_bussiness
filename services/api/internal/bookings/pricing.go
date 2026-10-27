// Package bookings creates and operates bookings (appointments, orders and rentals).
// Money is always integer centavos (MXN).
package bookings

import (
	"strings"
	"time"
	"unicode"
)

// NormalizePhone returns E.164 or "" if it can't be a phone. A bare 10-digit number is Mexican (+52).
func NormalizePhone(in string) string {
	plus := strings.HasPrefix(strings.TrimSpace(in), "+")
	var digits strings.Builder
	for _, r := range in {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case len(d) == 10 && !plus:
		return "+52" + d
	case len(d) >= 11 && len(d) <= 15 && d[0] != '0':
		return "+" + d
	default:
		return ""
	}
}

// RentalPeriods: how many price units a rental lasts. A started unit counts as a whole one.
// day = 24 h, weekend = up to 3 days (Fri–Mon), week = 7 days.
func RentalPeriods(unit string, d time.Duration) int64 {
	size := map[string]time.Duration{"day": 24 * time.Hour, "weekend": 72 * time.Hour, "week": 7 * 24 * time.Hour}[unit]
	if size == 0 || d <= 0 {
		return 1
	}
	return int64((d + size - 1) / size)
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
