package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

type Store interface {
	auth.Members
	Memberships(ctx context.Context, userID string) ([]store.Membership, error)
}

type Deps struct {
	Authenticate   func(http.Handler) http.Handler
	Store          Store
	AllowedOrigins []string // web origins of this environment; one "*" wildcard per entry allowed
	CreateBusiness func(ctx context.Context, userID string, in businesses.Input) (businesses.Business, error)
	Catalog        CatalogRepo
}

type CatalogRepo interface {
	List(ctx context.Context, businessID string) ([]catalog.Item, error)
	Get(ctx context.Context, businessID, itemID string) (catalog.Item, error)
	Create(ctx context.Context, businessID string, it catalog.Item) (catalog.Item, error)
	Update(ctx context.Context, businessID string, it catalog.Item) (catalog.Item, error)
	ListUnits(ctx context.Context, businessID, itemID string) ([]catalog.Unit, error)
	GetUnit(ctx context.Context, businessID, itemID, unitID string) (catalog.Unit, error)
	CreateUnit(ctx context.Context, businessID string, u catalog.Unit) (catalog.Unit, error)
	UpdateUnit(ctx context.Context, businessID string, u catalog.Unit) (catalog.Unit, error)
}

func notFound(w http.ResponseWriter, what string) {
	apierr.Write(w, http.StatusNotFound, "not_found", what)
}

// decode reads a JSON body (max 16 KiB); on failure it has already written the 400.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(v); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_json", "El cuerpo de la petición no es JSON válido.")
		return false
	}
	return true
}

func internalError(w http.ResponseWriter, r *http.Request, what string, err error) {
	slog.ErrorContext(r.Context(), what, "err", err)
	apierr.Write(w, http.StatusInternalServerError, "internal", "Error interno.")
}

func invalid(w http.ResponseWriter, message string, errs map[string]string) {
	apierr.JSON(w, http.StatusUnprocessableEntity, apierr.Error{Code: "validation_failed", Message: message, Details: errs})
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: d.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "Idempotency-Key"},
		MaxAge:         600,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Use(d.Authenticate)

		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.UserID(r.Context())
			ms, err := d.Store.Memberships(r.Context(), user)
			if err != nil {
				slog.ErrorContext(r.Context(), "memberships", "err", err)
				apierr.Write(w, http.StatusInternalServerError, "internal", "Error interno.")
				return
			}
			if ms == nil {
				ms = []store.Membership{}
			}
			apierr.JSON(w, http.StatusOK, map[string]any{"user_id": user, "businesses": ms})
		})

		r.Post("/businesses", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.UserID(r.Context())
			var in businesses.Input
			if !decode(w, r, &in) {
				return
			}
			in = in.Normalize()
			if errs := in.Validate(); len(errs) > 0 {
				invalid(w, "Revisa los datos del negocio.", errs)
				return
			}
			b, err := d.CreateBusiness(r.Context(), user, in)
			if errors.Is(err, businesses.ErrSlugTaken) {
				apierr.JSON(w, http.StatusConflict, apierr.Error{Code: "slug_taken", Message: "Esa dirección ya está en uso; elige otra.",
					Details: map[string]string{"slug": "Esa dirección ya está en uso; elige otra."}})
				return
			}
			if err != nil {
				internalError(w, r, "create business", err)
				return
			}
			apierr.JSON(w, http.StatusCreated, b)
		})

		r.Route("/b/{businessId}", func(r chi.Router) {
			r.Use(auth.RequireMember(d.Store))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				apierr.JSON(w, http.StatusOK, map[string]string{"business_id": m.BusinessID, "role": m.Role})
			})

			r.Get("/catalog", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				items, err := d.Catalog.List(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "list catalog", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"items": items})
			})

			r.With(auth.RequireRole("owner")).Post("/catalog", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				var it catalog.Item
				if !decode(w, r, &it) {
					return
				}
				it = it.Normalize()
				if errs := it.Validate(); len(errs) > 0 {
					invalid(w, "Revisa los datos del artículo.", errs)
					return
				}
				created, err := d.Catalog.Create(r.Context(), m.BusinessID, it)
				if err != nil {
					internalError(w, r, "create catalog item", err)
					return
				}
				apierr.JSON(w, http.StatusCreated, created)
			})

			r.With(auth.RequireRole("owner")).Patch("/catalog/{itemId}", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				var p catalog.Patch
				if !decode(w, r, &p) {
					return
				}
				itemID := chi.URLParam(r, "itemId")
				current, err := catalog.Item{}, catalog.ErrNotFound
				if auth.IsUUID(itemID) {
					current, err = d.Catalog.Get(r.Context(), m.BusinessID, itemID)
				}
				if errors.Is(err, catalog.ErrNotFound) {
					apierr.Write(w, http.StatusNotFound, "not_found", "El artículo no existe.")
					return
				}
				if err != nil {
					internalError(w, r, "get catalog item", err)
					return
				}
				it := current.Apply(p).Normalize()
				if errs := it.Validate(); len(errs) > 0 {
					invalid(w, "Revisa los datos del artículo.", errs)
					return
				}
				updated, err := d.Catalog.Update(r.Context(), m.BusinessID, it)
				if err != nil {
					internalError(w, r, "update catalog item", err)
					return
				}
				apierr.JSON(w, http.StatusOK, updated)
			})

			r.Get("/catalog/{itemId}/units", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				itemID := chi.URLParam(r, "itemId")
				units, err := []catalog.Unit(nil), error(catalog.ErrNotFound)
				if auth.IsUUID(itemID) {
					units, err = d.Catalog.ListUnits(r.Context(), m.BusinessID, itemID)
				}
				if errors.Is(err, catalog.ErrNotFound) {
					notFound(w, "La herramienta no existe.")
					return
				}
				if err != nil {
					internalError(w, r, "list units", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"units": units})
			})

			r.With(auth.RequireRole("owner")).Post("/catalog/{itemId}/units", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				itemID := chi.URLParam(r, "itemId")
				var p catalog.UnitPatch
				if !decode(w, r, &p) {
					return
				}
				u := catalog.Unit{ItemID: itemID, Status: "available"}.Apply(p)
				if errs := u.Validate(); len(errs) > 0 {
					invalid(w, "Revisa los datos de la unidad.", errs)
					return
				}
				created, err := catalog.Unit{}, error(catalog.ErrNotFound)
				if auth.IsUUID(itemID) {
					created, err = d.Catalog.CreateUnit(r.Context(), m.BusinessID, u)
				}
				writeUnit(w, r, http.StatusCreated, created, err)
			})

			r.With(auth.RequireRole("owner")).Patch("/catalog/{itemId}/units/{unitId}", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				itemID, unitID := chi.URLParam(r, "itemId"), chi.URLParam(r, "unitId")
				var p catalog.UnitPatch
				if !decode(w, r, &p) {
					return
				}
				current, err := catalog.Unit{}, error(catalog.ErrNotFound)
				if auth.IsUUID(itemID) && auth.IsUUID(unitID) {
					current, err = d.Catalog.GetUnit(r.Context(), m.BusinessID, itemID, unitID)
				}
				if err != nil {
					writeUnit(w, r, 0, current, err)
					return
				}
				u := current.Apply(p)
				if errs := u.Validate(); len(errs) > 0 {
					invalid(w, "Revisa los datos de la unidad.", errs)
					return
				}
				updated, err := d.Catalog.UpdateUnit(r.Context(), m.BusinessID, u)
				writeUnit(w, r, http.StatusOK, updated, err)
			})
		})
	})

	return r
}

func writeUnit(w http.ResponseWriter, r *http.Request, status int, u catalog.Unit, err error) {
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		notFound(w, "La herramienta o la unidad no existe.")
	case errors.Is(err, catalog.ErrCodeTaken):
		apierr.JSON(w, http.StatusConflict, apierr.Error{Code: "code_taken", Message: "Ya hay una unidad con esa clave.",
			Details: map[string]string{"code": "Ya hay una unidad con esa clave."}})
	case err != nil:
		internalError(w, r, "unit", err)
	default:
		apierr.JSON(w, status, u)
	}
}
