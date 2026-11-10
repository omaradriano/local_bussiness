package finances

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// CashLine: what moved by one payment method in a day. Deposits (rentals) are in the drawer even
// though they aren't income; refunds (of payments or deposits) went out of it.
type CashLine struct {
	Method   string `json:"method"`
	Orders   int64  `json:"orders"`   // advances, balances and fees of bookings
	Sales    int64  `json:"sales"`    // quick and product sales (not those left on an order's account)
	Deposits int64  `json:"deposits"` // security deposits taken
	Refunds  int64  `json:"refunds"`  // given back
	Net      int64  `json:"net"`
}

// CashClosing: the cash counted at the end of a day against what the report expected.
type CashClosing struct {
	Counted    int64     `json:"counted"`
	Expected   int64     `json:"expected"`
	Difference int64     `json:"difference"` // counted − expected: negative = missing
	Note       *string   `json:"note"`
	ClosedBy   *string   `json:"closed_by"` // who counted (name)
	ClosedAt   time.Time `json:"closed_at"`
}

type CashReport struct {
	Day          Day          `json:"day"`
	Lines        []CashLine   `json:"lines"`
	Total        int64        `json:"total"`
	ExpectedCash int64        `json:"expected_cash"` // the cash line's net: what should be in the drawer from the day
	Closing      *CashClosing `json:"closing"`
}

type cashRow struct {
	method, kind string
	amount       int64
}

// cashLines groups the day's movements by method, in a fixed order; methods without movement are left out.
func cashLines(rows []cashRow) []CashLine {
	by := map[string]*CashLine{}
	for _, r := range rows {
		l := by[r.method]
		if l == nil {
			l = &CashLine{Method: r.method}
			by[r.method] = l
		}
		switch r.kind {
		case "orders":
			l.Orders += r.amount
		case "sales":
			l.Sales += r.amount
		case "deposits":
			l.Deposits += r.amount
		case "refunds":
			l.Refunds += r.amount
		}
	}
	out := []CashLine{}
	for _, m := range []string{"cash", "transfer", "card"} {
		if l := by[m]; l != nil {
			l.Net = l.Orders + l.Sales + l.Deposits - l.Refunds
			out = append(out, *l)
		}
	}
	return out
}

func validateClosing(day Day, counted int64, note string, today Day) error {
	switch {
	case counted < 0 || counted > 100_000_000:
		return Invalid{"counted": "Escribe el efectivo contado (de $0 a $1,000,000)."}
	case day.After(today):
		return Invalid{"day": "No se puede hacer el corte de un día que no ha pasado."}
	case utf8.RuneCountInString(strings.TrimSpace(note)) > 200:
		return Invalid{"note": "La nota es muy larga (máximo 200 caracteres)."}
	}
	return nil
}

// dayBounds: a business date as instants [start, end) in its time zone.
func (r Repo) dayBounds(ctx context.Context, businessID string, day Day) (time.Time, time.Time, error) {
	var tz string
	if err := r.DB.QueryRow(ctx, `select timezone from public.businesses where id = $1`, businessID).Scan(&tz); err != nil {
		return time.Time{}, time.Time{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1), nil
}

// Cash is the day's report by method, with its closing if it was counted.
func (r Repo) Cash(ctx context.Context, businessID string, day Day) (CashReport, error) {
	out := CashReport{Day: day, Lines: []CashLine{}}
	start, end, err := r.dayBounds(ctx, businessID, day)
	if err != nil {
		return out, err
	}
	rows, err := r.DB.Query(ctx, `
		select method, case when type in ('payment_refund', 'refund') then 'refunds' when type = 'security_deposit' then 'deposits' else 'orders' end, sum(amount)::bigint
		from public.payments where business_id = $1 and paid_at >= $2 and paid_at < $3 group by 1, 2
		union all
		select method, 'sales', sum(total)::bigint
		from public.sales where business_id = $1 and sold_at >= $2 and sold_at < $3 and method <> 'on_account' group by 1`,
		businessID, start, end)
	if err != nil {
		return out, err
	}
	var list []cashRow
	for rows.Next() {
		var c cashRow
		if err := rows.Scan(&c.method, &c.kind, &c.amount); err != nil {
			rows.Close()
			return out, err
		}
		list = append(list, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.Lines = cashLines(list)
	for _, l := range out.Lines {
		out.Total += l.Net
		if l.Method == "cash" {
			out.ExpectedCash = l.Net
		}
	}
	var c CashClosing
	err = r.DB.QueryRow(ctx, `select k.counted, k.expected, k.note, s.name, k.closed_at
		from public.cash_closings k left join public.business_members m on m.business_id = k.business_id and m.user_id = k.closed_by
		  left join public.staff s on s.id = m.staff_id
		where k.business_id = $1 and k.day = $2`, businessID, day).Scan(&c.Counted, &c.Expected, &c.Note, &c.ClosedBy, &c.ClosedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return out, err
	default:
		c.Difference = c.Counted - c.Expected
		out.Closing = &c
	}
	return out, nil
}

// CloseCash records the cash counted for a day against what the report expects right now; counting
// again replaces it (a payment registered late changes what's expected).
func (r Repo) CloseCash(ctx context.Context, businessID string, day Day, counted int64, note, actor string, now time.Time) (CashReport, error) {
	today, err := r.Today(ctx, businessID, now)
	if err != nil {
		return CashReport{}, err
	}
	if err := validateClosing(day, counted, note, today); err != nil {
		return CashReport{}, err
	}
	rep, err := r.Cash(ctx, businessID, day)
	if err != nil {
		return rep, err
	}
	if _, err := r.DB.Exec(ctx, `insert into public.cash_closings (business_id, day, counted, expected, note, closed_by, closed_at)
		values ($1, $2, $3, $4, nullif($5, ''), nullif($6, '')::uuid, $7)
		on conflict (business_id, day) do update set counted = excluded.counted, expected = excluded.expected, note = excluded.note,
		  closed_by = excluded.closed_by, closed_at = excluded.closed_at`,
		businessID, day, counted, rep.ExpectedCash, strings.TrimSpace(note), actor, now); err != nil {
		return rep, err
	}
	return r.Cash(ctx, businessID, day)
}
