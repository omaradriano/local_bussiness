package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/bookings"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/staff"
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
	Staff          StaffRepo
	Photos         *Photos
	Availability   AvailabilityAnswerer
	Bookings       BookingsRepo
	Now            func() time.Time // injectable clock; nil = time.Now
}

type BookingsRepo interface {
	Create(ctx context.Context, businessID string, in bookings.CreateInput, now time.Time) (bookings.Booking, bool, error)
	Get(ctx context.Context, businessID, id string) (bookings.Booking, error)
	List(ctx context.Context, businessID string, f bookings.ListFilter) ([]bookings.Summary, error)
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

type AvailabilityAnswerer interface {
	Answer(ctx context.Context, businessID string, req availability.Request, now time.Time) (availability.Response, error)
}

type StaffRepo interface {
	List(ctx context.Context, businessID string) ([]staff.Member, error)
	Get(ctx context.Context, businessID, staffID string) (staff.Member, error)
	Create(ctx context.Context, businessID string, m staff.Member) (staff.Member, error)
	Update(ctx context.Context, businessID string, m staff.Member) error
	SetServices(ctx context.Context, businessID, staffID string, itemIDs []string) error
	SetHours(ctx context.Context, businessID, staffID string, hs []staff.Hours) error
	BusinessHours(ctx context.Context, businessID string) ([]staff.Hours, error)
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
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
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
				photos := map[string][]media.Image{}
				if d.Photos != nil && d.Photos.Media != nil {
					if photos, err = d.Photos.Media.ListForBusiness(r.Context(), m.BusinessID); err != nil {
						internalError(w, r, "list images", err)
						return
					}
				}
				for i := range items {
					imgs := []media.Image{}
					for _, img := range photos[items[i].ID] {
						imgs = append(imgs, img.WithURLs(d.Photos.CDNBase))
					}
					items[i].Images = imgs
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

			mountImages(r, d.Photos)

			// Counter booking: owner and staff can take it.
			r.Post("/bookings", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				var in bookings.CreateInput
				if !decode(w, r, &in) {
					return
				}
				if !auth.IsUUID(in.ItemID) || (in.StaffID != "" && !auth.IsUUID(in.StaffID)) || (in.Customer.ID != "" && !auth.IsUUID(in.Customer.ID)) {
					invalid(w, "Revisa los datos.", map[string]string{"item_id": "Elige un artículo válido."})
					return
				}
				in.Source, in.CreatedBy = "admin", m.UserID
				in.IdempotencyKey = r.Header.Get("Idempotency-Key")
				if k := in.IdempotencyKey; k != "" && (len(k) < 8 || len(k) > 100) {
					invalid(w, "Idempotency-Key inválida.", map[string]string{"idempotency_key": "Debe tener de 8 a 100 caracteres."})
					return
				}
				b, existed, err := d.Bookings.Create(r.Context(), m.BusinessID, in, d.now())
				var bad bookings.Invalid
				switch {
				case errors.As(err, &bad):
					invalid(w, bad.Message, map[string]string{bad.Field: bad.Message})
				case errors.Is(err, bookings.ErrSlotTaken):
					apierr.Write(w, http.StatusConflict, "slot_taken", "Ya no está disponible; elige otro horario, fecha o cantidad.")
				case err != nil:
					internalError(w, r, "create booking", err)
				case existed:
					apierr.JSON(w, http.StatusOK, b)
				default:
					apierr.JSON(w, http.StatusCreated, b)
				}
			})

			// Calendar / agenda: bookings overlapping [from, to), active ones by default.
			r.Get("/bookings", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				q := r.URL.Query()
				from, err1 := time.Parse(time.RFC3339, q.Get("from"))
				to, err2 := time.Parse(time.RFC3339, q.Get("to"))
				if err1 != nil || err2 != nil || !from.Before(to) || to.Sub(from) > 93*24*time.Hour {
					invalid(w, "Indica un rango válido (máximo 3 meses).", map[string]string{"from": "Rango inválido."})
					return
				}
				kind := q.Get("kind")
				if kind != "" && kind != "service" && kind != "product" && kind != "rental" {
					invalid(w, "Tipo inválido.", map[string]string{"kind": "Tipo inválido."})
					return
				}
				list, err := d.Bookings.List(r.Context(), m.BusinessID, bookings.ListFilter{From: from, To: to, Kind: kind, OnlyActive: q.Get("status") != "all"})
				if err != nil {
					internalError(w, r, "list bookings", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"bookings": list})
			})

			r.With(requireUUIDParam("bookingId", "La reserva no existe.")).Get("/bookings/{bookingId}", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				b, err := d.Bookings.Get(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"))
				switch {
				case errors.Is(err, bookings.ErrNotFound):
					notFound(w, "La reserva no existe.")
				case err != nil:
					internalError(w, r, "get booking", err)
				default:
					apierr.JSON(w, http.StatusOK, b)
				}
			})

			r.Get("/availability", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				q := r.URL.Query()
				req := availability.Request{ItemID: q.Get("item_id"), Date: q.Get("date"), From: q.Get("from"), To: q.Get("to"), StaffID: q.Get("staff_id")}
				if !auth.IsUUID(req.ItemID) || (req.StaffID != "" && !auth.IsUUID(req.StaffID)) {
					invalid(w, "Elige un artículo.", map[string]string{"item_id": "Elige un artículo válido."})
					return
				}
				if s := q.Get("quantity"); s != "" {
					n, err := strconv.Atoi(s)
					if err != nil || n < 1 || n > 100 {
						invalid(w, "Cantidad inválida.", map[string]string{"quantity": "La cantidad debe ser de 1 a 100."})
						return
					}
					req.Quantity = n
				}
				res, err := d.Availability.Answer(r.Context(), m.BusinessID, req, d.now())
				var bad availability.BadRequest
				switch {
				case errors.As(err, &bad):
					invalid(w, bad.Message, map[string]string{bad.Field: bad.Message})
				case errors.Is(err, availability.ErrItemNotFound):
					notFound(w, "El artículo no existe o está inactivo.")
				case err != nil:
					internalError(w, r, "availability", err)
				default:
					apierr.JSON(w, http.StatusOK, res)
				}
			})

			r.Get("/staff", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				list, err := d.Staff.List(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "list staff", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"staff": list})
			})

			r.With(auth.RequireRole("owner")).Post("/staff", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				var p staff.Patch
				if !decode(w, r, &p) {
					return
				}
				s := staff.Member{}.Apply(p)
				if errs := s.Validate(); len(errs) > 0 {
					invalid(w, "Revisa los datos.", errs)
					return
				}
				created, err := d.Staff.Create(r.Context(), m.BusinessID, s)
				if err != nil {
					internalError(w, r, "create staff", err)
					return
				}
				apierr.JSON(w, http.StatusCreated, created)
			})

			r.Route("/staff/{staffId}", func(r chi.Router) {
				r.Use(auth.RequireRole("owner"), requireUUIDParam("staffId", "La persona no existe."))

				r.Patch("/", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var p staff.Patch
					if !decode(w, r, &p) {
						return
					}
					current, err := d.Staff.Get(r.Context(), m.BusinessID, chi.URLParam(r, "staffId"))
					if errors.Is(err, staff.ErrNotFound) {
						notFound(w, "La persona no existe.")
						return
					}
					if err != nil {
						internalError(w, r, "get staff", err)
						return
					}
					s := current.Apply(p)
					if errs := s.Validate(); len(errs) > 0 {
						invalid(w, "Revisa los datos.", errs)
						return
					}
					if err := d.Staff.Update(r.Context(), m.BusinessID, s); err != nil {
						internalError(w, r, "update staff", err)
						return
					}
					apierr.JSON(w, http.StatusOK, s)
				})

				r.Put("/services", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						ServiceIDs []string `json:"service_ids"`
					}
					if !decode(w, r, &body) {
						return
					}
					for _, id := range body.ServiceIDs {
						if !auth.IsUUID(id) {
							invalid(w, "Revisa los servicios.", map[string]string{"service_ids": "Hay un servicio inválido."})
							return
						}
					}
					err := d.Staff.SetServices(r.Context(), m.BusinessID, chi.URLParam(r, "staffId"), body.ServiceIDs)
					switch {
					case errors.Is(err, staff.ErrNotFound):
						notFound(w, "La persona no existe.")
					case errors.Is(err, staff.ErrNotAService):
						invalid(w, "Revisa los servicios.", map[string]string{"service_ids": "Solo se pueden asignar servicios de este negocio."})
					case err != nil:
						internalError(w, r, "set staff services", err)
					default:
						w.WriteHeader(http.StatusNoContent)
					}
				})

				r.Put("/hours", func(w http.ResponseWriter, r *http.Request) {
					putHours(w, r, d.Staff, chi.URLParam(r, "staffId"))
				})
			})

			r.Get("/hours", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				hs, err := d.Staff.BusinessHours(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "business hours", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"hours": hs})
			})

			r.With(auth.RequireRole("owner")).Put("/hours", func(w http.ResponseWriter, r *http.Request) {
				putHours(w, r, d.Staff, "")
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

// putHours replaces the hours of a person (staffID) or of the business ("").
func putHours(w http.ResponseWriter, r *http.Request, repo StaffRepo, staffID string) {
	m, _ := auth.MemberFrom(r.Context())
	var body struct {
		Hours []staff.Hours `json:"hours"`
	}
	if !decode(w, r, &body) {
		return
	}
	if msg := staff.ValidateHours(body.Hours); msg != "" {
		invalid(w, msg, map[string]string{"hours": msg})
		return
	}
	err := repo.SetHours(r.Context(), m.BusinessID, staffID, body.Hours)
	switch {
	case errors.Is(err, staff.ErrNotFound):
		notFound(w, "La persona no existe.")
	case err != nil:
		internalError(w, r, "set hours", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// requireUUIDParam answers 404 when a path id is not a UUID (it can't exist), before touching the DB.
func requireUUIDParam(name, message string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !auth.IsUUID(chi.URLParam(r, name)) {
				notFound(w, message)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
