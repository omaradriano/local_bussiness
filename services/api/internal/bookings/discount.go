package bookings

import (
	"context"
	"fmt"
	"slices"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// Discount the business gives on a booking: a fixed amount in centavos or a whole percent of the
// total (with delivery, without the refundable security deposit). Type "" = none.
type Discount struct {
	Type  string `json:"type"`  // amount | percent
	Value int64  `json:"value"` // centavos (amount) or 0–100 (percent)
}

const maxDiscount = maxPayment // $1,000,000.00

func (d Discount) Validate() error {
	switch {
	case d.Type == "" && d.Value == 0:
		return nil
	case d.Type == "amount" && d.Value >= 0 && d.Value <= maxDiscount:
		return nil
	case d.Type == "percent" && d.Value >= 0 && d.Value <= 100:
		return nil
	case d.Type == "percent":
		return Invalid{"discount.value", "El porcentaje de descuento debe ser de 0 a 100."}
	case d.Type == "amount":
		return Invalid{"discount.value", "El descuento debe estar entre $0 y $1,000,000."}
	}
	return Invalid{"discount.type", "Elige si el descuento es una cantidad o un porcentaje."}
}

// Of is the amount it takes off a total, never more than the total.
func (d Discount) Of(total int64) int64 {
	var off int64
	switch d.Type {
	case "amount":
		off = d.Value
	case "percent":
		off = (total*d.Value + 50) / 100 // rounded to whole centavos
	}
	return min(max(off, 0), max(total, 0))
}

// rescaleAdvance keeps the advance's proportion when the total changes from `from` to `to`.
func rescaleAdvance(advance, from, to int64) int64 {
	if from <= 0 {
		return 0
	}
	return (advance*to + from/2) / from
}

// closedForDiscount: a finished or inactive booking keeps its total.
var closedForDiscount = []string{"completed", "cancelled", "expired", "no_show"}

// SetDiscount changes a booking's discount (Value 0 removes it): the total before the discount
// stays, the advance keeps its proportion, and it can't go below what was already charged.
func SetDiscount(ctx context.Context, db store.DB, businessID, id string, d Discount, actor string) (Booking, error) {
	if d.Value == 0 {
		d = Discount{}
	}
	if err := d.Validate(); err != nil {
		return Booking{}, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, id)
	if err != nil {
		return Booking{}, err
	}
	if slices.Contains(closedForDiscount, b.status) {
		return Booking{}, Invalid{"discount", fmt.Sprintf("Una reserva «%s» ya no cambia de total.", label(b.status))}
	}
	var current int64
	if err := tx.QueryRow(ctx, `select discount from public.bookings where id = $1`, id).Scan(&current); err != nil {
		return Booking{}, err
	}
	base := b.total + current // total before any discount
	off := d.Of(base)
	total := base - off
	ps, err := payments(ctx, tx, id)
	if err != nil {
		return Booking{}, err
	}
	if paid := ComputeBalance(b.total, ps).Paid; paid > total {
		return Booking{}, Invalid{"discount", fmt.Sprintf("Ya se cobró %s; el total con descuento no puede ser menor.", mxn(paid))}
	}
	if _, err := tx.Exec(ctx, `update public.bookings set discount_type = nullif($2, ''), discount_value = $3, discount = $4,
		total = $5, deposit_required = $6, updated_at = now() where id = $1`,
		id, d.Type, d.Value, off, total, rescaleAdvance(b.depositRequired, b.total, total)); err != nil {
		return Booking{}, err
	}
	note := "Sin descuento"
	if off > 0 {
		note = fmt.Sprintf("Descuento de %s; nuevo total %s", mxn(off), mxn(total))
	}
	if _, err := tx.Exec(ctx, `insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, note)
		values ($1, $2, $3, $3, nullif($4, '')::uuid, $5)`, businessID, id, b.status, actor, note); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
}
