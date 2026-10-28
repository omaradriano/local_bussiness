package bookings

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

// Document: a file attached to a booking with the name the business gave it (a contract, a note),
// besides the document types configured in Ajustes (those are steps).
type Document struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Files     []string  `json:"files"` // bucket private-docs
	CreatedAt time.Time `json:"created_at"`
}

// DocumentFilePrefix: free documents of the booking.
func DocumentFilePrefix(businessID, bookingID string) string {
	return FilePrefix(businessID, bookingID) + "documents/"
}

func documents(ctx context.Context, db store.DB, bookingID string) ([]Document, error) {
	rows, err := db.Query(ctx, `select id::text, name, s3_keys, created_at from public.booking_documents
		where booking_id = $1 order by created_at`, bookingID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Document])
	if out == nil {
		out = []Document{}
	}
	return out, err
}

// documentsLocked: once a booking is closed or fell through its documents stay as they are.
func documentsLocked(status string) bool {
	return status == "completed" || Releases(status)
}

func AddDocument(ctx context.Context, db store.DB, businessID, bookingID, name string, keys []string, actor string, exists FileExists) (Booking, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n == 0 || n > 60 {
		return Booking{}, Invalid{"name", "Escribe el nombre del documento (máximo 60 caracteres)."}
	}
	if len(keys) == 0 || len(keys) > 6 {
		return Booking{}, Invalid{"files", "Agrega de 1 a 6 archivos."}
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if documentsLocked(b.status) {
		return Booking{}, Invalid{"status", "Esta reserva ya está cerrada; no se le pueden agregar documentos."}
	}
	for _, k := range keys {
		if !strings.HasPrefix(k, DocumentFilePrefix(businessID, bookingID)) || strings.Contains(k, "..") {
			return Booking{}, Invalid{"files", "Hay un archivo que no pertenece a esta reserva."}
		}
		if err := exists(ctx, files.PrivateDocs, k); err != nil {
			return Booking{}, Invalid{"files", "Un archivo no se terminó de subir; vuelve a intentarlo."}
		}
	}
	if _, err := tx.Exec(ctx, `insert into public.booking_documents (business_id, booking_id, name, s3_keys, uploaded_by)
		values ($1, $2, $3, $4, nullif($5, '')::uuid)`, businessID, bookingID, name, keys, actor); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return Get(ctx, db, businessID, bookingID)
}

// RemoveDocument deletes one free document; returns its keys so the caller can delete the objects.
func RemoveDocument(ctx context.Context, db store.DB, businessID, bookingID, docID string) (Booking, []string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return Booking{}, nil, err
	}
	defer tx.Rollback(ctx)
	b, err := lockBooking(ctx, tx, businessID, bookingID)
	if err != nil {
		return Booking{}, nil, err
	}
	if documentsLocked(b.status) {
		return Booking{}, nil, Invalid{"status", "Esta reserva ya está cerrada; sus documentos no se pueden quitar."}
	}
	var keys []string
	err = tx.QueryRow(ctx, `delete from public.booking_documents where booking_id = $1 and id = $2 returning s3_keys`, bookingID, docID).Scan(&keys)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, nil, ErrNotFound
	}
	if err != nil {
		return Booking{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, nil, err
	}
	bk, err := Get(ctx, db, businessID, bookingID)
	return bk, keys, err
}
