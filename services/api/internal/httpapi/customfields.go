package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/customfields"
)

type CustomFieldsRepo interface {
	List(ctx context.Context, businessID, kind string) ([]customfields.Field, error)
	Get(ctx context.Context, businessID, id string) (customfields.Field, error)
	Create(ctx context.Context, businessID string, f customfields.Field) (customfields.Field, error)
	Update(ctx context.Context, businessID string, f customfields.Field) (customfields.Field, error)
	Delete(ctx context.Context, businessID, id string) error
}

// mountCustomFields: the owner defines the fields (module customFields); every member reads them.
func mountCustomFields(r chi.Router, d Deps) {
	r.Get("/custom-fields", func(w http.ResponseWriter, r *http.Request) {
		m, _ := auth.MemberFrom(r.Context())
		list, err := d.CustomFields.List(r.Context(), m.BusinessID, r.URL.Query().Get("kind"))
		if err != nil {
			internalError(w, r, "list custom fields", err)
			return
		}
		apierr.JSON(w, http.StatusOK, map[string]any{"fields": list})
	})

	r.With(auth.RequireRole("owner"), d.requireFeature("customFields")).Post("/custom-fields", func(w http.ResponseWriter, r *http.Request) {
		m, _ := auth.MemberFrom(r.Context())
		var f customfields.Field
		if !decode(w, r, &f) {
			return
		}
		f = f.Normalize()
		if errs := f.Validate(); len(errs) > 0 {
			invalid(w, "Revisa el campo.", errs)
			return
		}
		created, err := d.CustomFields.Create(r.Context(), m.BusinessID, f)
		writeField(w, r, http.StatusCreated, created, err)
	})

	r.Route("/custom-fields/{fieldId}", func(r chi.Router) {
		r.Use(auth.RequireRole("owner"), requireUUIDParam("fieldId", "El campo no existe."))

		r.With(d.requireFeature("customFields")).Patch("/", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var p customfields.Patch
			if !decode(w, r, &p) {
				return
			}
			current, err := d.CustomFields.Get(r.Context(), m.BusinessID, chi.URLParam(r, "fieldId"))
			if err != nil {
				writeField(w, r, 0, current, err)
				return
			}
			f := current.Apply(p).Normalize()
			if errs := f.Validate(); len(errs) > 0 {
				invalid(w, "Revisa el campo.", errs)
				return
			}
			updated, err := d.CustomFields.Update(r.Context(), m.BusinessID, f)
			writeField(w, r, http.StatusOK, updated, err)
		})

		r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			err := d.CustomFields.Delete(r.Context(), m.BusinessID, chi.URLParam(r, "fieldId"))
			writeField(w, r, http.StatusNoContent, customfields.Field{}, err)
		})
	})
}

func writeField(w http.ResponseWriter, r *http.Request, status int, f customfields.Field, err error) {
	switch {
	case errors.Is(err, customfields.ErrNotFound):
		notFound(w, "El campo no existe.")
	case err != nil:
		internalError(w, r, "custom field", err)
	case status == http.StatusNoContent:
		w.WriteHeader(status)
	default:
		apierr.JSON(w, status, f)
	}
}

// customValues validates the custom values sent for an item or a unit ("item" | "unit") of an engine.
// Nothing sent = the saved values stay. ok is false when it already answered (422 or 500).
func (d Deps) customValues(w http.ResponseWriter, r *http.Request, kind, target string, sent *catalog.Values, saved catalog.Values) (catalog.Values, bool) {
	if sent == nil || d.CustomFields == nil {
		return saved, true
	}
	m, _ := auth.MemberFrom(r.Context())
	fields, err := d.CustomFields.List(r.Context(), m.BusinessID, kind)
	if err != nil {
		internalError(w, r, "custom fields", err)
		return nil, false
	}
	clean, errs := customfields.Clean(fields, target, *sent, saved)
	if len(errs) > 0 {
		invalid(w, "Revisa los campos personalizados.", errs)
		return nil, false
	}
	return clean, true
}
