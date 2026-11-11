package finances

import (
	"context"
	"time"
)

// Receivable: a booking that still owes money.
type Receivable struct {
	BookingID string    `json:"booking_id"`
	Customer  string    `json:"customer"`
	Phone     string    `json:"phone"`
	StartsAt  time.Time `json:"starts_at"`
	Status    string    `json:"status"`
	Total     int64     `json:"total"` // with extras on its account
	Due       int64     `json:"due"`
}

// Receivables: what customers owe and the advances already taken for what hasn't been handed over.
type Receivables struct {
	Credit       []Receivable `json:"credit"`        // handed over on credit, still owing
	Active       []Receivable `json:"active"`        // not handed over yet, with a balance (by date)
	TotalDue     int64        `json:"total_due"`     // both lists
	Advances     int64        `json:"advances"`      // paid toward bookings not handed over yet (owed in goods)
	AdvanceCount int          `json:"advance_count"` // how many bookings those advances are for
}

// open: not handed over and not fallen through.
const openStatuses = `('pending_deposit', 'confirmed', 'in_progress', 'ready', 'out_for_delivery', 'checked_out', 'overdue', 'returned')`

func (r Repo) Receivables(ctx context.Context, businessID string) (Receivables, error) {
	out := Receivables{Credit: []Receivable{}, Active: []Receivable{}}
	rows, err := r.DB.Query(ctx, `
		with owed as (
		  select b.id, b.status, b.starts_at, coalesce(b.walk_in_name, c.name) as customer, coalesce(b.walk_in_phone, c.phone) as phone,
		    b.total + coalesce((select sum(s.total) from public.sales s where s.booking_id = b.id and s.method = 'on_account'), 0) as total,
		    coalesce((select sum(case when p.type = 'payment_refund' then -p.amount else p.amount end) from public.payments p
		              where p.booking_id = b.id and p.type in ('advance', 'balance', 'payment_refund')), 0) as paid
		  from public.bookings b join public.customers c on c.id = b.customer_id
		  where b.business_id = $1 and (b.status = 'completed' or b.status in `+openStatuses+`))
		select id::text, customer, phone, starts_at, status, total, total - paid, paid from owed
		where total - paid > 0 or (status <> 'completed' and paid > 0)
		order by starts_at limit 1000`, businessID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Receivable
		var paid int64
		if err := rows.Scan(&x.BookingID, &x.Customer, &x.Phone, &x.StartsAt, &x.Status, &x.Total, &x.Due, &paid); err != nil {
			return out, err
		}
		if x.Status != "completed" && paid > 0 {
			out.Advances += min(paid, x.Total)
			out.AdvanceCount++
		}
		if x.Due <= 0 {
			continue
		}
		out.TotalDue += x.Due
		if x.Status == "completed" {
			out.Credit = append(out.Credit, x)
		} else {
			out.Active = append(out.Active, x)
		}
	}
	return out, rows.Err()
}
