package bookings

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// PartialRefundInput: part of what was paid goes back (damaged flowers) without cancelling the order.
type PartialRefundInput struct {
	Amount int64  `json:"amount"`
	Method string `json:"method"`
	Reason string `json:"reason"`
}

// Validate against the order's total and what's been paid toward it (net of what was given back).
func (r PartialRefundInput) Validate(total, paid int64) error {
	switch {
	case !slices.Contains(paymentMethods, r.Method):
		return Invalid{"method", "Elige efectivo, transferencia o tarjeta."}
	case r.Amount <= 0:
		return Invalid{"amount", "Escribe un monto mayor a cero."}
	case r.Amount > total:
		return Invalid{"amount", fmt.Sprintf("No puede ser mayor que el total del pedido (%s).", mxn(total))}
	case r.Amount > paid:
		return Invalid{"amount", fmt.Sprintf("No se puede devolver más de lo cobrado (%s).", mxn(paid))}
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(r.Reason)); n == 0 || n > 200 {
		return Invalid{"reason", "Escribe el motivo (máximo 200 caracteres)."}
	}
	return nil
}

// PartialRefund gives part of the payment back: a payment_refund (it comes off the income) and the
// total goes down by the same amount, so what's still owed doesn't change.
func PartialRefund(ctx context.Context, db store.DB, businessID, id string, in PartialRefundInput, actor string) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, id)
	if err != nil {
		return Booking{}, err
	}
	if Releases(b.status) {
		return Booking{}, Invalid{"amount", "Este pedido ya está cancelado; lo cobrado se resolvió al cancelarlo."}
	}
	ps, err := payments(ctx, tx, id)
	if err != nil {
		return Booking{}, err
	}
	if err := in.Validate(b.total, ComputeBalance(b.total+b.onAccount, ps).Paid); err != nil {
		return Booking{}, err
	}
	reason := strings.TrimSpace(in.Reason)
	if _, err := tx.Exec(ctx, `insert into public.payments (business_id, booking_id, type, method, amount, paid_at, recorded_by, notes)
		values ($1, $2, 'payment_refund', $3, $4, now(), nullif($5, '')::uuid, $6)`, businessID, id, in.Method, in.Amount, actor, reason); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `update public.bookings set total = total - $2, deposit_required = least(deposit_required, total - $2), updated_at = now()
		where id = $1`, id, in.Amount); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, note)
		values ($1, $2, $3, $3, nullif($4, '')::uuid, $5)`, businessID, id, b.status, actor,
		fmt.Sprintf("Devolución parcial de %s: %s; nuevo total %s", mxn(in.Amount), reason, mxn(b.total-in.Amount))); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
}
