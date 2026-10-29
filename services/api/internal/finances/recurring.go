package finances

import (
	"slices"
	"strings"
	"unicode/utf8"
)

var Frequencies = []string{"weekly", "biweekly", "monthly", "yearly"}

// Recurring: an expense charged every so often (rent, internet, electricity). Its amount is an
// estimate: each charge stays pending until the owner confirms the real amount (it becomes an
// Expense) or says it wasn't paid.
type Recurring struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Amount      int64  `json:"amount"`
	Frequency   string `json:"frequency"`
	StartsOn    Day    `json:"starts_on"`
	EndsOn      *Day   `json:"ends_on"`
}

// Due is the i-th charge (0 = StartsOn). Months and years keep the day, clamped to the month's end;
// biweekly is that day and 15 days later, every month (a Mexican «quincena»: the 1st and the 16th).
func (r Recurring) Due(i int) Day {
	switch r.Frequency {
	case "weekly":
		return r.StartsOn.AddDate(0, 0, 7*i)
	case "biweekly":
		return addMonths(r.StartsOn, i/2).AddDate(0, 0, 15*(i%2))
	case "yearly":
		return addMonths(r.StartsOn, 12*i)
	default:
		return addMonths(r.StartsOn, i)
	}
}

func (r Recurring) ended(day Day) bool { return r.EndsOn != nil && day.After(*r.EndsOn) }

// DueIn lists the charges in [from, to].
func (r Recurring) DueIn(from, to Day) []Day {
	var out []Day
	for i := 0; ; i++ {
		due := r.Due(i)
		if due.After(to) || r.ended(due) {
			return out
		}
		if !due.Before(from) {
			out = append(out, due)
		}
	}
}

// IsDue reports whether day is one of its charges.
func (r Recurring) IsDue(day Day) bool {
	for _, due := range r.DueIn(day, day) {
		if due.Equal(day) {
			return true
		}
	}
	return false
}

// Pending: charges up to today with no answer (answered: "YYYY-MM-DD" -> paid or skipped), oldest
// first. Only the last year is asked for, so an old recurring expense doesn't flood the list.
func (r Recurring) Pending(today Day, answered map[string]bool) []Day {
	var out []Day
	for _, due := range r.DueIn(today.AddDate(-1, 0, 0), today) {
		if !answered[due.Format("2006-01-02")] {
			out = append(out, due)
		}
	}
	return out
}

// NextDue: the first charge after today (nil once it ended).
func (r Recurring) NextDue(today Day) *Day {
	for i := 0; ; i++ {
		due := r.Due(i)
		if r.ended(due) {
			return nil
		}
		if due.After(today) {
			return &due
		}
	}
}

func (r Recurring) Validate() error {
	errs := Invalid{}
	if n := utf8.RuneCountInString(strings.TrimSpace(r.Description)); n == 0 || n > 120 {
		errs["description"] = "Escribe qué se paga (máximo 120 caracteres)."
	}
	if !slices.Contains(Categories, r.Category) {
		errs["category"] = "Elige una categoría."
	}
	if r.Amount <= 0 || r.Amount > maxAmount {
		errs["amount"] = "Escribe un monto mayor a cero."
	}
	if !slices.Contains(Frequencies, r.Frequency) {
		errs["frequency"] = "Elige cada cuánto se paga."
	}
	if r.StartsOn.IsZero() {
		errs["starts_on"] = "Elige desde cuándo se paga."
	}
	if r.EndsOn != nil && r.EndsOn.Before(r.StartsOn) {
		errs["ends_on"] = "La fecha final debe ser después de la inicial."
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
