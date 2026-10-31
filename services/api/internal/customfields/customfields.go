// Package customfields: fields the owner adds to the catalog (an engine module). Each one has a
// name, a type (text, number, list, date, yes/no) and belongs to the item (brand, model) or to each
// unit (serial number, condition) when the engine has units. Values live in custom_values (jsonb,
// key = field id) of catalog_items and item_units; Clean validates them against the definitions.
package customfields

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var Types = []string{"text", "number", "select", "date", "boolean"}

var ErrNotFound = errors.New("custom field not found")

type Field struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`   // engine it applies to
	Target   string   `json:"target"` // item | unit
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Options  []string `json:"options"` // select only
	Required bool     `json:"required"`
	Active   bool     `json:"active"`
	Sort     int      `json:"sort"`
}

// Patch: nil = unchanged. The engine, the target and the type don't change (the saved values
// depend on them): to change them, delete the field and create another.
type Patch struct {
	Label    *string   `json:"label"`
	Options  *[]string `json:"options"`
	Required *bool     `json:"required"`
	Active   *bool     `json:"active"`
	Sort     *int      `json:"sort"`
}

func (f Field) Apply(p Patch) Field {
	if p.Label != nil {
		f.Label = *p.Label
	}
	if p.Options != nil {
		f.Options = *p.Options
	}
	if p.Required != nil {
		f.Required = *p.Required
	}
	if p.Active != nil {
		f.Active = *p.Active
	}
	if p.Sort != nil {
		f.Sort = *p.Sort
	}
	return f
}

// Normalize trims the text and drops the options of a type that isn't a list.
func (f Field) Normalize() Field {
	f.Label = strings.Join(strings.Fields(f.Label), " ")
	if f.Type != "select" {
		f.Options = []string{}
		return f
	}
	opts := make([]string, len(f.Options))
	for i, o := range f.Options {
		opts[i] = strings.Join(strings.Fields(o), " ")
	}
	f.Options = opts
	return f
}

// Validate returns field -> message (es-MX).
func (f Field) Validate() map[string]string {
	errs := map[string]string{}
	if !engines.Valid(f.Kind) {
		errs["kind"] = "Tipo de catálogo inválido."
	}
	switch {
	case f.Target != "item" && f.Target != "unit":
		errs["target"] = "Elige si el campo es del artículo o de cada unidad."
	case f.Target == "unit" && !catalog.HasUnits(f.Kind):
		errs["target"] = "Este catálogo no tiene unidades; el campo debe ser del artículo."
	}
	if n := utf8.RuneCountInString(f.Label); n == 0 || n > 60 {
		errs["label"] = "Escribe el nombre del campo (máximo 60 caracteres)."
	}
	if !slices.Contains(Types, f.Type) {
		errs["type"] = "Elige el tipo de dato."
	}
	if f.Type == "select" {
		seen := map[string]bool{}
		for _, o := range f.Options {
			k := strings.ToLower(o)
			if o == "" || utf8.RuneCountInString(o) > 40 || seen[k] {
				errs["options"] = "Cada opción debe tener texto (máximo 40 caracteres) y no repetirse."
				break
			}
			seen[k] = true
		}
		if len(f.Options) == 0 || len(f.Options) > 30 {
			errs["options"] = "Agrega de 1 a 30 opciones."
		}
	}
	return errs
}

const maxNumber = 1e12

// Clean checks the values sent for one target ("item" or "unit") against the business's fields:
// keeps the valid ones of active fields (trimmed and typed), drops empty and unknown keys, keeps the
// saved value of inactive fields, and returns "custom_values.<id>" -> message for the bad ones.
func Clean(fields []Field, target string, in, saved map[string]any) (map[string]any, map[string]string) {
	out := map[string]any{}
	errs := map[string]string{}
	for _, f := range fields {
		if f.Target != target {
			continue
		}
		if !f.Active {
			if v, ok := saved[f.ID]; ok {
				out[f.ID] = v
			}
			continue
		}
		key := "custom_values." + f.ID
		v, msg := clean(f, in[f.ID])
		switch {
		case msg != "":
			errs[key] = msg
		case v != nil:
			out[f.ID] = v
		case f.Required:
			errs[key] = fmt.Sprintf("Escribe %s.", strings.ToLower(f.Label))
		}
	}
	return out, errs
}

// clean returns the value to store (nil = empty) or an error message.
func clean(f Field, v any) (any, string) {
	if v == nil {
		return nil, ""
	}
	if s, ok := v.(string); ok && f.Type != "boolean" {
		if s = strings.TrimSpace(s); s == "" {
			return nil, ""
		}
		v = s
	}
	switch f.Type {
	case "text":
		if s, ok := v.(string); ok && utf8.RuneCountInString(s) <= 200 {
			return s, ""
		}
		return nil, fmt.Sprintf("%s: texto de máximo 200 caracteres.", f.Label)
	case "number":
		if n, ok := v.(float64); ok && !math.IsNaN(n) && math.Abs(n) <= maxNumber {
			return n, ""
		}
		return nil, fmt.Sprintf("%s: escribe un número.", f.Label)
	case "select":
		if s, ok := v.(string); ok && slices.Contains(f.Options, s) {
			return s, ""
		}
		return nil, fmt.Sprintf("%s: elige una de las opciones.", f.Label)
	case "date":
		if s, ok := v.(string); ok {
			if d, err := time.Parse("2006-01-02", s); err == nil && d.Format("2006-01-02") == s {
				return s, ""
			}
		}
		return nil, fmt.Sprintf("%s: fecha inválida (AAAA-MM-DD).", f.Label)
	case "boolean":
		if b, ok := v.(bool); ok {
			return b, ""
		}
		return nil, fmt.Sprintf("%s: elige sí o no.", f.Label)
	}
	return nil, ""
}

// Repo reads and writes the definitions; every query is scoped to the business.
type Repo struct{ DB store.DB }

const columns = `id::text, applies_to_kind, target, label, type, options, required, active, sort`

func scan(row pgx.Row) (Field, error) {
	var f Field
	err := row.Scan(&f.ID, &f.Kind, &f.Target, &f.Label, &f.Type, &f.Options, &f.Required, &f.Active, &f.Sort)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	if f.Options == nil {
		f.Options = []string{}
	}
	return f, err
}

// List returns the business's fields of an engine ("" = every engine), in order.
func (r Repo) List(ctx context.Context, businessID, kind string) ([]Field, error) {
	rows, err := r.DB.Query(ctx, `select `+columns+` from public.custom_fields
		where business_id = $1 and ($2 = '' or applies_to_kind = $2) order by sort, created_at`, businessID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Field{}
	for rows.Next() {
		f, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r Repo) Get(ctx context.Context, businessID, id string) (Field, error) {
	return scan(r.DB.QueryRow(ctx, `select `+columns+` from public.custom_fields where business_id = $1 and id = $2`, businessID, id))
}

func (r Repo) Create(ctx context.Context, businessID string, f Field) (Field, error) {
	return scan(r.DB.QueryRow(ctx, `
		insert into public.custom_fields (business_id, applies_to_kind, target, label, type, options, required, active, sort)
		values ($1, $2, $3, $4, $5, $6, $7, true,
		        coalesce((select max(sort) + 1 from public.custom_fields where business_id = $1), 1))
		returning `+columns, businessID, f.Kind, f.Target, f.Label, f.Type, f.Options, f.Required))
}

func (r Repo) Update(ctx context.Context, businessID string, f Field) (Field, error) {
	return scan(r.DB.QueryRow(ctx, `
		update public.custom_fields set label = $3, type = $4, options = $5, required = $6, active = $7, sort = $8
		where business_id = $1 and id = $2
		returning `+columns, businessID, f.ID, f.Label, f.Type, f.Options, f.Required, f.Active, f.Sort))
}

// Delete removes the definition; saved values under its id are no longer shown nor validated.
func (r Repo) Delete(ctx context.Context, businessID, id string) error {
	tag, err := r.DB.Exec(ctx, `delete from public.custom_fields where business_id = $1 and id = $2`, businessID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
