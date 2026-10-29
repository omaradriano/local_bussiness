// Package finances keeps the owner's money picture: expenses, purchases in installments, the
// team's pay, and a summary of income vs expenses for any period. Cash basis: income is what was
// collected in the period (security deposits are held, not income).
package finances

import "time"

// Day is a calendar date (midnight UTC) so date math has no time zone surprises.
type Day = time.Time

// ParseDay reads "2026-11-03".
func ParseDay(s string) (Day, bool) {
	d, err := time.Parse(time.DateOnly, s)
	return d, err == nil
}

// addMonths keeps the day of the month, clamped to the month's length (31 Jan + 1 → 28/29 Feb).
func addMonths(d Day, n int) Day {
	first := time.Date(d.Year(), d.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d.Day(), last)-1)
}

// Plan is a purchase paid in installments (a truck): down payment on the purchase day, then one
// installment a month from FirstDue.
type Plan struct {
	ID                string `json:"id"`
	Description       string `json:"description"`
	Category          string `json:"category"`
	PurchasedOn       Day    `json:"purchased_on"`
	DownPayment       int64  `json:"down_payment"`
	InstallmentAmount int64  `json:"installment_amount"`
	Installments      int    `json:"installments"`
	FirstDue          Day    `json:"first_due"`
}

// Due is the date of installment i (0-based).
func (p Plan) Due(i int) Day { return addMonths(p.FirstDue, i) }

// InRange: what the plan costs inside [from, to] (down payment + installments due then).
func (p Plan) InRange(from, to Day) int64 {
	var total int64
	if !p.PurchasedOn.Before(from) && !p.PurchasedOn.After(to) {
		total += p.DownPayment
	}
	for i := range p.Installments {
		d := p.Due(i)
		if d.After(to) {
			break
		}
		if !d.Before(from) {
			total += p.InstallmentAmount
		}
	}
	return total
}

// Progress: installments already due by today, what's still owed and the next due date (nil when done).
func (p Plan) Progress(today Day) (paid int, remaining int64, next *Day) {
	for i := range p.Installments {
		if d := p.Due(i); !d.After(today) {
			paid++
		} else if next == nil {
			next = &d
		}
	}
	return paid, int64(p.Installments-paid) * p.InstallmentAmount, next
}

// Fixed pay per day for a salary period (a year has 52 weeks, 26 fortnights, 12 months).
func perDay(salary int64, period string) float64 {
	perYear := map[string]float64{"weekly": 52, "biweekly": 26, "monthly": 12}[period]
	return float64(salary) * perYear / 365
}

// ProratedSalary: fixed pay earned in [from, to], counting only days up to today (no future pay).
func ProratedSalary(salary int64, period string, from, to, today Day) int64 {
	if to.After(today) {
		to = today
	}
	if to.Before(from) || salary <= 0 {
		return 0
	}
	days := int(to.Sub(from).Hours()/24) + 1
	return int64(perDay(salary, period)*float64(days) + 0.5)
}

// Commission: percent of what was collected for the person's services, rounded to centavos.
func Commission(collected float64, percent float64) int64 {
	return int64(collected*percent/100 + 0.5)
}
