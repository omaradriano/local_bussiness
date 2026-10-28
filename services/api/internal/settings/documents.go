package settings

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// DocumentType: an extra document a business asks for in every rental (e.g. "Pagaré"), shown in
// the check-out's "Documentos adicionales". It's a workflow step (file_upload before check-out,
// config.section = "additional"), so each new rental copies the active ones.
type DocumentType struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Required bool   `json:"required"` // the check-out waits for it
	Active   bool   `json:"active"`
}

type DocumentTypeInput struct {
	Name     *string `json:"name"`
	Required *bool   `json:"required"`
	Active   *bool   `json:"active"`
}

// Apply merges the input; the name is trimmed.
func (d DocumentType) Apply(in DocumentTypeInput) DocumentType {
	if in.Name != nil {
		d.Name = strings.Join(strings.Fields(*in.Name), " ")
	}
	if in.Required != nil {
		d.Required = *in.Required
	}
	if in.Active != nil {
		d.Active = *in.Active
	}
	return d
}

func (d DocumentType) Validate() map[string]string {
	if n := utf8.RuneCountInString(d.Name); n == 0 || n > 60 {
		return map[string]string{"name": "Escribe el nombre del documento (máximo 60 caracteres)."}
	}
	return nil
}

const docCols = `id::text, label, required, active`
const docWhere = `business_id = $1 and applies_to_kind = 'rental' and config->>'section' = 'additional'`

func (r Repo) DocumentTypes(ctx context.Context, businessID string) ([]DocumentType, error) {
	rows, err := r.DB.Query(ctx, `select `+docCols+` from public.workflow_steps where `+docWhere+` order by sort, label`, businessID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[DocumentType])
	if out == nil {
		out = []DocumentType{}
	}
	return out, err
}

func (r Repo) CreateDocumentType(ctx context.Context, businessID string, d DocumentType) (DocumentType, error) {
	var out DocumentType
	err := r.DB.QueryRow(ctx, `insert into public.workflow_steps
		(business_id, applies_to_kind, actor, moment, to_status, step_type, label, config, required, active, sort)
		values ($1, 'rental', 'business', 'before_status', 'checked_out', 'file_upload', $2,
		        '{"bucket": "private-docs", "max_files": 6, "section": "additional"}', $3, $4,
		        coalesce((select max(sort) from public.workflow_steps where business_id = $1), 0) + 1)
		returning `+docCols, businessID, d.Name, d.Required, d.Active).
		Scan(&out.ID, &out.Name, &out.Required, &out.Active)
	return out, err
}

func (r Repo) DocumentType(ctx context.Context, businessID, id string) (DocumentType, error) {
	var d DocumentType
	err := r.DB.QueryRow(ctx, `select `+docCols+` from public.workflow_steps where `+docWhere+` and id = $2`, businessID, id).
		Scan(&d.ID, &d.Name, &d.Required, &d.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// UpdateDocumentType changes the name, required or active; rentals already created keep their copy.
func (r Repo) UpdateDocumentType(ctx context.Context, businessID string, d DocumentType) (DocumentType, error) {
	_, err := r.DB.Exec(ctx, `update public.workflow_steps set label = $3, required = $4, active = $5
		where `+docWhere+` and id = $2`, businessID, d.ID, d.Name, d.Required, d.Active)
	return d, err
}
