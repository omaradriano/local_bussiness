package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCodeTaken = errors.New("unit code taken")

// Unit is one physical piece of a rental item (e.g. "RM-01").
type Unit struct {
	ID     string `json:"id"`
	ItemID string `json:"item_id"`
	Code   string `json:"code"`
	Status string `json:"status"` // available | maintenance | retired
	Notes  string `json:"notes"`
}

type UnitPatch struct {
	Code   *string `json:"code"`
	Status *string `json:"status"`
	Notes  *string `json:"notes"`
}

func (u Unit) Apply(p UnitPatch) Unit {
	if p.Code != nil {
		u.Code = *p.Code
	}
	if p.Status != nil {
		u.Status = *p.Status
	}
	if p.Notes != nil {
		u.Notes = *p.Notes
	}
	u.Code = strings.ToUpper(strings.TrimSpace(u.Code))
	u.Notes = strings.TrimSpace(u.Notes)
	return u
}

func (u Unit) Validate() map[string]string {
	errs := map[string]string{}
	if n := utf8.RuneCountInString(u.Code); n == 0 || n > 40 {
		errs["code"] = "Escribe una clave para la unidad (máximo 40 caracteres), por ejemplo RM-01."
	}
	if !slices.Contains([]string{"available", "maintenance", "retired"}, u.Status) {
		errs["status"] = "Estado inválido."
	}
	return errs
}

const unitColumns = `id::text, item_id::text, code, status, coalesce(notes, '')`

func scanUnit(row pgx.Row) (Unit, error) {
	var u Unit
	err := row.Scan(&u.ID, &u.ItemID, &u.Code, &u.Status, &u.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return u, ErrCodeTaken
	}
	return u, err
}

// ListUnits returns the units of a rental item of the business (ErrNotFound if it isn't one).
func (r Repo) ListUnits(ctx context.Context, businessID, itemID string) ([]Unit, error) {
	var isRental bool
	err := r.DB.QueryRow(ctx, `select kind = 'rental' from public.catalog_items where business_id = $1 and id = $2`, businessID, itemID).Scan(&isRental)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !isRental) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.DB.Query(ctx, `select `+unitColumns+` from public.item_units where business_id = $1 and item_id = $2 order by code`, businessID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := []Unit{}
	for rows.Next() {
		u, err := scanUnit(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

func (r Repo) CreateUnit(ctx context.Context, businessID string, u Unit) (Unit, error) {
	// Only inserts when the item is a rental of this business; otherwise no row -> ErrNotFound.
	return scanUnit(r.DB.QueryRow(ctx, `
		insert into public.item_units (business_id, item_id, code, status, notes)
		select i.business_id, i.id, $3, $4, nullif($5, '')
		from public.catalog_items i
		where i.business_id = $1 and i.id = $2 and i.kind = 'rental'
		returning `+unitColumns, businessID, u.ItemID, u.Code, u.Status, u.Notes))
}

func (r Repo) GetUnit(ctx context.Context, businessID, itemID, unitID string) (Unit, error) {
	return scanUnit(r.DB.QueryRow(ctx, `select `+unitColumns+` from public.item_units where business_id = $1 and item_id = $2 and id = $3`,
		businessID, itemID, unitID))
}

func (r Repo) UpdateUnit(ctx context.Context, businessID string, u Unit) (Unit, error) {
	return scanUnit(r.DB.QueryRow(ctx, `
		update public.item_units set code = $4, status = $5, notes = nullif($6, '')
		where business_id = $1 and item_id = $2 and id = $3
		returning `+unitColumns, businessID, u.ItemID, u.ID, u.Code, u.Status, u.Notes))
}
