package bookings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/steps"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// FilePrefix: every private file of a booking lives under this key prefix (ID, evidence photos).
func FilePrefix(businessID, bookingID string) string {
	return businessID + "/bookings/" + bookingID + "/"
}

// PaymentFilePrefix: evidence photos of the booking's payments.
func PaymentFilePrefix(businessID, bookingID string) string {
	return FilePrefix(businessID, bookingID) + "payments/"
}

// NewFileKey returns a fresh key for an upload of the booking.
func NewFileKey(businessID, bookingID, ext string) string {
	return FilePrefix(businessID, bookingID) + RandomName(ext)
}

// RandomName: an unguessable file name with that extension.
func RandomName(ext string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b) + "." + ext
}

// StepUpload resolves where a step's files go; ErrNotFound if the step isn't the booking's or takes no files.
func StepUpload(ctx context.Context, db store.DB, businessID, bookingID, stepID string) (bucket string, err error) {
	var stepType string
	var config json.RawMessage
	err = db.QueryRow(ctx, `select coalesce(step_snapshot->>'step_type', ''), coalesce(step_snapshot->'config', '{}')
		from public.booking_steps where business_id = $1 and booking_id = $2 and id = $3`, businessID, bookingID, stepID).Scan(&stepType, &config)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	st, ok := steps.Lookup(stepType)
	if !ok || st.Bucket(config) == "" {
		return "", ErrNotFound
	}
	return st.Bucket(config), nil
}

// FileExists checks that an uploaded object is really there (bucket, key).
type FileExists func(ctx context.Context, bucket, key string) error

// CompleteStep validates a submission with the step's registered type and marks it done.
func CompleteStep(ctx context.Context, db store.DB, businessID, bookingID, stepID string, sub steps.Submission, actor string, now time.Time, exists FileExists) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	bk, err := lockBooking(ctx, tx, businessID, bookingID)
	if err != nil {
		return Booking{}, err
	}
	var stepType, status, toStatus string
	var config json.RawMessage
	var existing []string
	err = tx.QueryRow(ctx, `select coalesce(step_snapshot->>'step_type', ''), coalesce(step_snapshot->'config', '{}'), status,
		coalesce(step_snapshot->>'to_status', ''), s3_keys
		from public.booking_steps where booking_id = $1 and id = $2 for update`, bookingID, stepID).Scan(&stepType, &config, &status, &toStatus, &existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	st, ok := steps.Lookup(stepType)
	if !ok {
		return Booking{}, Invalid{"step", "Este tipo de paso todavía no se puede completar desde la app."}
	}
	// A done photo step takes more photos while its moment hasn't passed (e.g. out-photos until check-out).
	appending := false
	if status == "done" {
		canAdd := st.Bucket(config) != "" && len(sub.Files) > 0 && len(sub.Data) == 0 && toStatus != "" && StillAhead(bk.kind, bk.status, toStatus)
		if !canAdd {
			return Booking{}, Invalid{"step", "Este paso ya está completo."}
		}
		appending = true
	}
	prefix := FilePrefix(businessID, bookingID)
	for _, k := range sub.Files {
		if !strings.HasPrefix(k, prefix) || strings.Contains(k, "..") {
			return Booking{}, Invalid{"files", "Hay un archivo que no pertenece a esta reserva."}
		}
	}
	newFiles := sub.Files
	if appending {
		sub.Files = append(append([]string{}, existing...), newFiles...) // the type validates the whole set (max files)
	}
	if err := st.ValidateData(config, sub); err != nil {
		var ue steps.UserError
		if errors.As(err, &ue) {
			return Booking{}, Invalid{"data", ue.Error()}
		}
		return Booking{}, err
	}
	for _, k := range newFiles {
		if err := exists(ctx, st.Bucket(config), k); err != nil {
			return Booking{}, Invalid{"files", "Una foto no se terminó de subir; vuelve a intentarlo."}
		}
	}
	data := sub.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	if sub.Files == nil {
		sub.Files = []string{}
	}
	if appending {
		if _, err := tx.Exec(ctx, `update public.booking_steps set s3_keys = $3 where booking_id = $1 and id = $2`, bookingID, stepID, sub.Files); err != nil {
			return Booking{}, err
		}
	} else if _, err := tx.Exec(ctx, `update public.booking_steps set status = 'done', data = $3, s3_keys = $4,
		completed_by = nullif($5, '')::uuid, completed_at = $6 where booking_id = $1 and id = $2`,
		bookingID, stepID, data, sub.Files, actor, now); err != nil {
		return Booking{}, err
	}
	// A customer document (ID, proof of address) also goes to the customer's profile, so the
	// next rental doesn't ask for it again.
	if _, err := tx.Exec(ctx, `
		insert into public.customer_documents (business_id, customer_id, doc_type, s3_key, uploaded_by)
		select b.business_id, b.customer_id, bs.step_snapshot->'config'->>'customer_doc', k, nullif($3, '')::uuid
		from public.booking_steps bs join public.bookings b on b.id = bs.booking_id, unnest($4::text[]) k
		where bs.booking_id = $1 and bs.id = $2 and bs.step_snapshot->'config'->>'customer_doc' is not null
		on conflict (s3_key) do nothing`, bookingID, stepID, actor, newFiles); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, bookingID)
}

// RemoveStepFile takes one photo out of a step while its moment hasn't passed (a wrong ID photo,
// a blurry picture). With no photos left the step is pending again. A customer document goes
// out of the customer's profile too. Returns the removed key so the caller can delete the object.
func RemoveStepFile(ctx context.Context, db store.DB, businessID, bookingID, stepID, key string) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	bk, err := lockBooking(ctx, tx, businessID, bookingID)
	if err != nil {
		return Booking{}, err
	}
	var toStatus string
	var files []string
	err = tx.QueryRow(ctx, `select coalesce(step_snapshot->>'to_status', ''), s3_keys from public.booking_steps
		where booking_id = $1 and id = $2 for update`, bookingID, stepID).Scan(&toStatus, &files)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if !slices.Contains(files, key) {
		return Booking{}, ErrNotFound
	}
	if toStatus == "" || !StillAhead(bk.kind, bk.status, toStatus) {
		return Booking{}, Invalid{"step", "Este paso ya no se puede cambiar."}
	}
	rest := slices.DeleteFunc(slices.Clone(files), func(k string) bool { return k == key })
	if _, err := tx.Exec(ctx, `update public.booking_steps set s3_keys = $3,
		status = case when cardinality($3::text[]) = 0 then 'pending' else status end,
		completed_at = case when cardinality($3::text[]) = 0 then null else completed_at end,
		completed_by = case when cardinality($3::text[]) = 0 then null else completed_by end
		where booking_id = $1 and id = $2`, bookingID, stepID, rest); err != nil {
		return Booking{}, err
	}
	if _, err := tx.Exec(ctx, `delete from public.customer_documents where business_id = $1 and s3_key = $2`, businessID, key); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, bookingID)
}

// Checkout hands the units to the customer: needs the booking confirmed, the steps before
// "checked_out" done (ID, photos) and the security deposit held.
func Checkout(ctx context.Context, db store.DB, businessID, bookingID, condition, actor string, now time.Time) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if b.kind != "rental" || !CanTransition(b.kind, b.status, "checked_out") {
		return Booking{}, Invalid{"status", fmt.Sprintf("No se puede registrar la salida de una renta «%s».", label(b.status))}
	}
	if err := satisfyFromCustomer(ctx, tx, bookingID, now); err != nil {
		return Booking{}, err
	}
	if pending, err := pendingSteps(ctx, tx, bookingID, "checked_out"); err != nil || len(pending) > 0 {
		if err != nil {
			return Booking{}, err
		}
		return Booking{}, StepsPending{pending}
	}
	var required int64
	if err := tx.QueryRow(ctx, `select coalesce(sum((options->>'security_deposit')::bigint), 0) from public.booking_items where booking_id = $1`, bookingID).Scan(&required); err != nil {
		return Booking{}, err
	}
	ps, err := payments(ctx, tx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if held := ComputeBalance(b.total, ps).DepositHeld; held < required {
		return Booking{}, Invalid{"status", fmt.Sprintf("Falta registrar el depósito de garantía (%s).", mxn(required-held))}
	}
	// Evidence photos taken for the check-out steps go with every unit's record.
	var photos []string
	if err := tx.QueryRow(ctx, `select coalesce(array_agg(k), '{}') from public.booking_steps, unnest(s3_keys) k
		where booking_id = $1 and step_snapshot->>'to_status' = 'checked_out'`, bookingID).Scan(&photos); err != nil {
		return Booking{}, err
	}
	tag, err := tx.Exec(ctx, `insert into public.rental_checkouts (business_id, booking_id, item_unit_id, out_at, out_condition, out_photos)
		select business_id, booking_id, item_unit_id, $2, nullif($3, ''), $4 from public.booking_resources
		where booking_id = $1 and item_unit_id is not null`, bookingID, now, strings.TrimSpace(condition), photos)
	if err != nil {
		return Booking{}, err
	}
	if tag.RowsAffected() == 0 {
		return Booking{}, Invalid{"status", "La renta no tiene unidades asignadas."}
	}
	if err := setStatus(ctx, tx, businessID, bookingID, b.status, "checked_out", actor, condition); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, bookingID)
}

// Return records the units back. If they come back early the units are free from now on;
// the late fee is suggested in the booking detail (RentalInfo), charged as a payment.
func Return(ctx context.Context, db store.DB, businessID, bookingID, condition, actor string, now time.Time) (Booking, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if b.kind != "rental" || !CanTransition(b.kind, b.status, "returned") {
		return Booking{}, Invalid{"status", fmt.Sprintf("No se puede registrar la devolución de una renta «%s».", label(b.status))}
	}
	if _, err := tx.Exec(ctx, `update public.rental_checkouts set returned_at = $2, return_condition = nullif($3, '')
		where booking_id = $1 and returned_at is null`, bookingID, now, strings.TrimSpace(condition)); err != nil {
		return Booking{}, err
	}
	// Early return frees the units for others. A late one keeps its original range (the late
	// time can't be claimed if someone else already booked it); it's charged as a late fee.
	if _, err := tx.Exec(ctx, `update public.booking_resources set during = tstzrange(lower(during), $2)
		where booking_id = $1 and $2 > lower(during) and $2 < upper(during)`, bookingID, now); err != nil {
		return Booking{}, err
	}
	if err := setStatus(ctx, tx, businessID, bookingID, b.status, "returned", actor, condition); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, bookingID)
}

// LateDays: whole days (started ones count) between the due return and the actual one, with an hour of grace.
func LateDays(due, returned time.Time) int64 {
	late := returned.Sub(due) - time.Hour
	if late <= 0 {
		return 0
	}
	return int64((late + 24*time.Hour - 1) / (24 * time.Hour))
}

// satisfyFromCustomer completes the pending "customer document" steps (config.customer_doc) with
// what's already in the customer's profile; a trusted customer satisfies them all.
func satisfyFromCustomer(ctx context.Context, tx pgx.Tx, bookingID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		update public.booking_steps bs
		set status = 'done', completed_at = $2, data = '{"from_customer": true}',
		    s3_keys = coalesce((select array_agg(cd.s3_key order by cd.created_at desc) from public.customer_documents cd
		                        where cd.customer_id = b.customer_id and cd.doc_type = bs.step_snapshot->'config'->>'customer_doc'), '{}')
		from public.bookings b
		join public.customers c on c.id = b.customer_id
		where bs.booking_id = $1 and b.id = $1 and bs.status = 'pending'
		  and bs.step_snapshot->'config' ? 'customer_doc'
		  and (c.trusted or exists (select 1 from public.customer_documents cd
		                            where cd.customer_id = b.customer_id and cd.doc_type = bs.step_snapshot->'config'->>'customer_doc'))`,
		bookingID, now)
	return err
}
