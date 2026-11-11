package customers

import (
	"context"
	"time"
)

// Recipient: someone the customer sends orders to (giftOrder), grouped by name ignoring case and spaces.
type Recipient struct {
	Name   string    `json:"name"` // as written the last time
	Phone  *string   `json:"phone"`
	Count  int       `json:"count"`
	LastAt time.Time `json:"last_at"`
}

// Summary of a customer's history with the business; cancelled, expired and no-show bookings don't count.
type Summary struct {
	Orders     int         `json:"orders"`
	Spent      int64       `json:"spent"` // paid, net of refunds, plus extras paid at the counter with their orders
	Due        int64       `json:"due"`   // still owed (on credit or not handed over yet), extras on account included
	FirstAt    *time.Time  `json:"first_at"`
	LastAt     *time.Time  `json:"last_at"`
	Recipients []Recipient `json:"recipients"` // the 5 most frequent
}

func (r Repo) Summary(ctx context.Context, businessID, customerID string) (Summary, error) {
	out := Summary{Recipients: []Recipient{}}
	const counted = `b.business_id = $1 and b.customer_id = $2 and b.status not in ('cancelled', 'expired', 'no_show')`
	if err := r.DB.QueryRow(ctx, `
		with x as (
		  select b.starts_at,
		    b.total + coalesce((select sum(s.total) from public.sales s where s.booking_id = b.id and s.method = 'on_account'), 0) as owed,
		    coalesce((select sum(case when p.type = 'payment_refund' then -p.amount else p.amount end) from public.payments p
		              where p.booking_id = b.id and p.type in ('advance', 'balance', 'late_fee', 'damage_fee', 'payment_refund')), 0) as paid,
		    coalesce((select sum(s.total) from public.sales s where s.booking_id = b.id and s.method <> 'on_account'), 0) as extras
		  from public.bookings b where `+counted+`)
		select count(*)::int, coalesce(sum(paid + extras), 0)::bigint, coalesce(sum(greatest(owed - paid, 0)), 0)::bigint, min(starts_at), max(starts_at) from x`,
		businessID, customerID).Scan(&out.Orders, &out.Spent, &out.Due, &out.FirstAt, &out.LastAt); err != nil {
		return out, err
	}
	rows, err := r.DB.Query(ctx, `
		select (array_agg(trim(b.recipient_name) order by b.starts_at desc))[1], (array_agg(b.recipient_phone order by b.starts_at desc))[1],
		  count(*)::int, max(b.starts_at)
		from public.bookings b where `+counted+` and coalesce(trim(b.recipient_name), '') <> ''
		group by lower(trim(b.recipient_name)) order by 3 desc, 4 desc limit 5`, businessID, customerID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Recipient
		if err := rows.Scan(&x.Name, &x.Phone, &x.Count, &x.LastAt); err != nil {
			return out, err
		}
		out.Recipients = append(out.Recipients, x)
	}
	return out, rows.Err()
}
