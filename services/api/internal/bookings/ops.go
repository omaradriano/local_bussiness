package bookings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// StepsPending: required steps must be completed before moving to that status.
type StepsPending struct{ Labels []string }

func (e StepsPending) Error() string {
	return "Faltan pasos obligatorios: " + strings.Join(e.Labels, ", ") + "."
}

// HandledElsewhere: rental check-out and return go through their own flow (units, photos, fees).
var HandledElsewhere = Invalid{"status", "La salida y la devolución de una renta se registran desde su propio formulario."}

var statusLabel = map[string]string{
	"pending_deposit": "Pendiente de anticipo", "confirmed": "Confirmada", "in_progress": "En proceso", "ready": "Lista",
	"out_for_delivery": "En camino", "checked_out": "Rentada", "overdue": "Vencida", "returned": "Devuelta",
	"completed": "Completada", "cancelled": "Cancelada", "expired": "Expirada", "no_show": "No asistió",
}

type locked struct {
	kind, status, fulfillment string
	total, depositRequired    int64
	onAccount                 int64 // extras on the order's account (R4): owed with it
}

func lockBooking(ctx context.Context, tx pgx.Tx, businessID, id string) (locked, error) {
	var l locked
	err := tx.QueryRow(ctx, `select kind, status, coalesce(fulfillment, ''), total, deposit_required,
		  (select coalesce(sum(s.total), 0) from public.sales s where s.booking_id = b.id and s.method = 'on_account')
		from public.bookings b where business_id = $1 and id = $2 for update of b`, businessID, id).
		Scan(&l.kind, &l.status, &l.fulfillment, &l.total, &l.depositRequired, &l.onAccount)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrNotFound
	}
	return l, err
}

func payments(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, bookingID string) ([]Payment, error) {
	rows, err := q.Query(ctx, `select id::text, type, method, amount, paid_at, recorded_by::text, notes, evidence_keys
		from public.payments where booking_id = $1 order by paid_at, created_at`, bookingID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Payment])
	if out == nil {
		out = []Payment{}
	}
	return out, err
}

// setStatus moves the booking and logs it; releasing statuses give the slot/units back.
func setStatus(ctx context.Context, tx pgx.Tx, businessID, id, from, to, actor, note string) error {
	if _, err := tx.Exec(ctx, `update public.bookings set status = $3, updated_at = now(),
		hold_expires_at = case when $3 = 'pending_deposit' then hold_expires_at end
		where business_id = $1 and id = $2`, businessID, id, to); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, note)
		values ($1, $2, $3, $4, nullif($5, '')::uuid, nullif($6, ''))`, businessID, id, from, to, actor, note); err != nil {
		return err
	}
	if Releases(to) {
		_, err := tx.Exec(ctx, `delete from public.booking_resources where booking_id = $1`, id)
		return err
	}
	return nil
}

// Transition applies a person's status change after checking the machine and the requirements.
// What happens to the money already paid when a booking with payments is cancelled (or no-show).
const (
	PaidRefund = "refund" // given back: a payment_refund, it comes off the income
	PaidKeep   = "keep"   // kept as a penalty: stays as income
)

func Transition(ctx context.Context, db store.DB, businessID, id, to, actor, note string) (Booking, error) {
	return TransitionPaid(ctx, db, businessID, id, to, "", actor, note)
}

// TransitionPaid is Transition saying what to do with what was paid when it releases the booking;
// a releasing move with payments requires that choice.
func TransitionPaid(ctx context.Context, db store.DB, businessID, id, to, paid, actor, note string) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, id)
	if err != nil {
		return Booking{}, err
	}
	if !ByPeople(to) || !CanTransition(b.kind, b.status, to) {
		return Booking{}, Invalid{"status", "Desde su estado actual ya no se puede hacer ese cambio."}
	}
	if hasCustody(b.kind) && (to == "checked_out" || to == "returned") {
		return Booking{}, HandledElsewhere
	}
	ps, err := payments(ctx, tx, id)
	if err != nil {
		return Booking{}, err
	}
	bal := ComputeBalance(b.total+b.onAccount, ps)
	switch {
	case to == "confirmed" && bal.Paid == 0 && b.depositRequired > 0: // with part of the advance, the business may confirm it
		return Booking{}, Invalid{"status", fmt.Sprintf("Registra al menos un pago del anticipo (%s).", mxn(b.depositRequired))}
	case to == "completed" && bal.Due > 0:
		return Booking{}, Invalid{"status", fmt.Sprintf("Falta cobrar el saldo (%s).", mxn(bal.Due))}
	case to == "out_for_delivery" && b.fulfillment != "delivery":
		return Booking{}, Invalid{"status", "Este pedido se recoge en el local; no sale a entrega."}
	}
	if to == "confirmed" && bal.Paid < b.depositRequired {
		note = joinNote(note, fmt.Sprintf("Se confirmó con anticipo parcial (%s de %s)", mxn(bal.Paid), mxn(b.depositRequired)))
	}
	var extras int64 // linked quick sales already paid (1.3) follow the same choice as what was paid toward the order
	if Releases(to) {
		if err := tx.QueryRow(ctx, `select coalesce(sum(total), 0) from public.sales where business_id = $1 and booking_id = $2 and method <> 'on_account'`,
			businessID, id).Scan(&extras); err != nil {
			return Booking{}, err
		}
		// Extras on the order's account (R4) were never delivered: they go back to stock either way.
		if err := undoSales(ctx, tx, businessID, id, true); err != nil {
			return Booking{}, err
		}
	}
	if collected := bal.Paid + extras; Releases(to) && collected > 0 {
		switch paid {
		case PaidRefund:
			for _, r := range PaidByMethod(ps) {
				if _, err := tx.Exec(ctx, `insert into public.payments (business_id, booking_id, type, method, amount, paid_at, recorded_by, notes)
					values ($1, $2, 'payment_refund', $3, $4, now(), nullif($5, '')::uuid, 'Devolución al cancelar')`,
					businessID, id, r.Method, r.Amount, actor); err != nil {
					return Booking{}, err
				}
			}
			if err := undoSales(ctx, tx, businessID, id, false); err != nil {
				return Booking{}, err
			}
			note = joinNote(note, fmt.Sprintf("Se devolvieron %s", mxn(collected)))
		case PaidKeep:
			note = joinNote(note, fmt.Sprintf("Se retienen %s como penalización", mxn(collected)))
		default:
			return Booking{}, Invalid{"paid", fmt.Sprintf("Ya se cobraron %s: elige si se devuelven o se retienen.", mxn(collected))}
		}
	}
	if pending, err := pendingSteps(ctx, tx, id, to); err != nil || len(pending) > 0 {
		if err != nil {
			return Booking{}, err
		}
		return Booking{}, StepsPending{pending}
	}
	if err := setStatus(ctx, tx, businessID, id, b.status, to, actor, note); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
}

// undoSales deletes the quick sales linked to the booking (only those on its account, or all) and puts
// their products' stock back, like products.CancelSale.
func undoSales(ctx context.Context, tx pgx.Tx, businessID, bookingID string, onAccount bool) error {
	if _, err := tx.Exec(ctx, `update public.retail_products p set stock = p.stock + q.quantity
		from (select l.product_id, sum(l.quantity) as quantity from public.sale_lines l join public.sales s on s.id = l.sale_id
		      where s.business_id = $1 and s.booking_id = $2 and l.product_id is not null and (not $3 or s.method = 'on_account')
		      group by l.product_id) q
		where p.id = q.product_id and p.business_id = $1`, businessID, bookingID, onAccount); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `delete from public.sales where business_id = $1 and booking_id = $2 and (not $3 or method = 'on_account')`, businessID, bookingID, onAccount)
	return err
}

// pendingSteps lists the required steps still pending before reaching `to`.
func pendingSteps(ctx context.Context, tx pgx.Tx, bookingID, to string) ([]string, error) {
	rows, err := tx.Query(ctx, `select step_snapshot->>'label' from public.booking_steps
		where booking_id = $1 and status = 'pending'
		  and step_snapshot->>'moment' = 'before_status' and step_snapshot->>'to_status' = $2
		  and coalesce((step_snapshot->>'required')::boolean, true)
		order by id`, bookingID, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// AddPayment records money for the booking. Paying the full advance of a hold confirms it.
func AddPayment(ctx context.Context, db store.DB, businessID, id string, p Payment, actor string, now time.Time, exists FileExists) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, id)
	if err != nil {
		return Booking{}, err
	}
	if Releases(b.status) && p.Type != "refund" {
		return Booking{}, Invalid{"type", "En su estado actual solo se pueden registrar devoluciones."}
	}
	if err := PaymentAllowed(b.kind, b.status, p.Type); err != nil {
		return Booking{}, err
	}
	if len(p.Evidence) > 4 {
		return Booking{}, Invalid{"evidence", "Máximo 4 fotos de evidencia."}
	}
	for _, k := range p.Evidence {
		if !strings.HasPrefix(k, PaymentFilePrefix(businessID, id)) || strings.Contains(k, "..") {
			return Booking{}, Invalid{"evidence", "Hay un archivo que no pertenece a esta reserva."}
		}
		if err := exists(ctx, files.PrivateDocs, k); err != nil {
			return Booking{}, Invalid{"evidence", "Una foto no se terminó de subir; vuelve a intentarlo."}
		}
	}
	if p.Evidence == nil {
		p.Evidence = []string{}
	}
	ps, err := payments(ctx, tx, id)
	if err != nil {
		return Booking{}, err
	}
	bal := ComputeBalance(b.total+b.onAccount, ps)
	if err := ValidatePayment(p, bal); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `insert into public.payments (business_id, booking_id, type, method, amount, paid_at, recorded_by, notes, evidence_keys)
		values ($1, $2, $3, $4, $5, $6, nullif($7, '')::uuid, nullif($8, ''), $9)`,
		businessID, id, p.Type, p.Method, p.Amount, now, actor, strings.TrimSpace(deref(p.Notes)), p.Evidence); err != nil {
		return Booking{}, err
	}
	if b.status == "pending_deposit" && (p.Type == "advance" || p.Type == "balance") {
		if bal.Paid+p.Amount >= b.depositRequired {
			if err := setStatus(ctx, tx, businessID, id, b.status, "confirmed", actor, "Anticipo registrado"); err != nil {
				return Booking{}, err
			}
		} else if _, err := tx.Exec(ctx, `update public.bookings set hold_expires_at = null where id = $1`, id); err != nil {
			return Booking{}, err // part of the advance: the hold no longer expires
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
}

func joinNote(note, add string) string {
	if note = strings.TrimSpace(note); note == "" {
		return add
	}
	return note + " · " + add
}

func label(status string) string {
	if l, ok := statusLabel[status]; ok {
		return l
	}
	return status
}

func mxn(centavos int64) string {
	pesos, cents := centavos/100, centavos%100
	s := fmt.Sprintf("%d", pesos)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return fmt.Sprintf("$%s.%02d", s, cents)
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
