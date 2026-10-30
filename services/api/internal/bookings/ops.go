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
}

func lockBooking(ctx context.Context, tx pgx.Tx, businessID, id string) (locked, error) {
	var l locked
	err := tx.QueryRow(ctx, `select kind, status, coalesce(fulfillment, ''), total, deposit_required
		from public.bookings where business_id = $1 and id = $2 for update`, businessID, id).
		Scan(&l.kind, &l.status, &l.fulfillment, &l.total, &l.depositRequired)
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
func Transition(ctx context.Context, db store.DB, businessID, id, to, actor, note string) (Booking, error) {
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
		return Booking{}, Invalid{"status", fmt.Sprintf("No se puede pasar de «%s» a «%s».", statusLabel[b.status], label(to))}
	}
	if hasCustody(b.kind) && (to == "checked_out" || to == "returned") {
		return Booking{}, HandledElsewhere
	}
	ps, err := payments(ctx, tx, id)
	if err != nil {
		return Booking{}, err
	}
	bal := ComputeBalance(b.total, ps)
	switch {
	case to == "confirmed" && bal.Paid < b.depositRequired:
		return Booking{}, Invalid{"status", fmt.Sprintf("Falta registrar el anticipo (%s).", mxn(b.depositRequired-bal.Paid))}
	case to == "completed" && bal.Due > 0:
		return Booking{}, Invalid{"status", fmt.Sprintf("Falta cobrar el saldo (%s).", mxn(bal.Due))}
	case to == "out_for_delivery" && b.fulfillment != "delivery":
		return Booking{}, Invalid{"status", "Este pedido se recoge en el local; no sale a entrega."}
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
		return Booking{}, Invalid{"type", fmt.Sprintf("La reserva está «%s»; solo se pueden registrar devoluciones.", statusLabel[b.status])}
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
	bal := ComputeBalance(b.total, ps)
	if err := ValidatePayment(p, bal); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `insert into public.payments (business_id, booking_id, type, method, amount, paid_at, recorded_by, notes, evidence_keys)
		values ($1, $2, $3, $4, $5, $6, nullif($7, '')::uuid, nullif($8, ''), $9)`,
		businessID, id, p.Type, p.Method, p.Amount, now, actor, strings.TrimSpace(deref(p.Notes)), p.Evidence); err != nil {
		return Booking{}, err
	}
	if b.status == "pending_deposit" && (p.Type == "advance" || p.Type == "balance") && bal.Paid+p.Amount >= b.depositRequired {
		if err := setStatus(ctx, tx, businessID, id, b.status, "confirmed", actor, "Anticipo registrado"); err != nil {
			return Booking{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, id)
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
