package customers

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Invalid: field → message (es-MX) for a date that can't be saved.
type Invalid map[string]string

func (e Invalid) Error() string { return "invalid customer date" }

// Date: an important date of a customer that comes back every year (a birthday, an anniversary).
type Date struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Month int    `json:"month"`
	Day   int    `json:"day"`
}

func (d Date) Validate() error {
	if n := utf8.RuneCountInString(strings.TrimSpace(d.Label)); n == 0 || n > 80 {
		return Invalid{"label": "Escribe qué se celebra (máximo 80 caracteres)."}
	}
	// 2024 is a leap year, so 29 Feb is a valid date to keep.
	if d.Month < 1 || d.Month > 12 || d.Day < 1 || time.Date(2024, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC).Day() != d.Day {
		return Invalid{"day": "Esa fecha no existe."}
	}
	return nil
}

// NextOccurrence: the next time the date comes, from today (included), and how many days away.
// 29 Feb falls on the 28th in years without it.
func NextOccurrence(month, day int, today time.Time) (time.Time, int) {
	on := func(year int) time.Time {
		d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if d.Month() != time.Month(month) { // 29 Feb rolled into March
			d = time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
		}
		return d
	}
	next := on(today.Year())
	if next.Before(today) {
		next = on(today.Year() + 1)
	}
	return next, int(next.Sub(today).Hours() / 24)
}

// Upcoming: a customer's date that comes within the next days.
type Upcoming struct {
	Date
	CustomerID   string    `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	Phone        string    `json:"phone"`
	On           time.Time `json:"on"`
	DaysLeft     int       `json:"days_left"`
}

func (r Repo) Dates(ctx context.Context, businessID, customerID string) ([]Date, error) {
	rows, err := r.DB.Query(ctx, `select id::text, label, month, day from public.customer_dates
		where business_id = $1 and customer_id = $2 order by month, day`, businessID, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Date{}
	for rows.Next() {
		var d Date
		if err := rows.Scan(&d.ID, &d.Label, &d.Month, &d.Day); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r Repo) AddDate(ctx context.Context, businessID, customerID string, d Date) (Date, error) {
	d.Label = strings.Join(strings.Fields(d.Label), " ")
	if err := d.Validate(); err != nil {
		return d, err
	}
	err := r.DB.QueryRow(ctx, `insert into public.customer_dates (business_id, customer_id, label, month, day)
		values ($1, $2, $3, $4, $5) returning id::text`, businessID, customerID, d.Label, d.Month, d.Day).Scan(&d.ID)
	return d, err
}

func (r Repo) DeleteDate(ctx context.Context, businessID, customerID, id string) error {
	tag, err := r.DB.Exec(ctx, `delete from public.customer_dates where business_id = $1 and customer_id = $2 and id::text = $3`, businessID, customerID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// UpcomingDates: the business's customer dates within `days` from today (business date), soonest first.
func (r Repo) UpcomingDates(ctx context.Context, businessID string, today time.Time, days int) ([]Upcoming, error) {
	// shortcut: reads every date of the business and filters in Go; fine for hundreds, move to SQL if it grows.
	rows, err := r.DB.Query(ctx, `select d.id::text, d.label, d.month, d.day, c.id::text, c.name, c.phone
		from public.customer_dates d join public.customers c on c.id = d.customer_id where d.business_id = $1`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Upcoming{}
	for rows.Next() {
		var u Upcoming
		if err := rows.Scan(&u.ID, &u.Label, &u.Month, &u.Day, &u.CustomerID, &u.CustomerName, &u.Phone); err != nil {
			return nil, err
		}
		if u.On, u.DaysLeft = NextOccurrence(u.Month, u.Day, today); u.DaysLeft <= days {
			out = append(out, u)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DaysLeft < out[j].DaysLeft })
	return out, rows.Err()
}
