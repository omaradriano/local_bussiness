package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/availability"
	"github.com/omaradriano/local_bussiness/services/api/internal/bookings"
	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
	"github.com/omaradriano/local_bussiness/services/api/internal/exports"
	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	"github.com/omaradriano/local_bussiness/services/api/internal/files"
	"github.com/omaradriano/local_bussiness/services/api/internal/finances"
	"github.com/omaradriano/local_bussiness/services/api/internal/imports"
	"github.com/omaradriano/local_bussiness/services/api/internal/media"
	"github.com/omaradriano/local_bussiness/services/api/internal/monitor"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
	"github.com/omaradriano/local_bussiness/services/api/internal/platform"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
	"github.com/omaradriano/local_bussiness/services/api/internal/settings"
	"github.com/omaradriano/local_bussiness/services/api/internal/staff"
	"github.com/omaradriano/local_bussiness/services/api/internal/steps"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

type Store interface {
	auth.Members
	Memberships(ctx context.Context, userID string) ([]store.Membership, error)
	IsPlatformAdmin(ctx context.Context, userID string) (bool, error)
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
	Settings       SettingsRepo
	Customers      CustomersRepo
	Platform       PlatformRepo
	Finances       FinancesRepo
	Products       ProductsRepo
	Plans          PlansRepo
	// DeleteBusiness removes a business, its rows and (by a job) its files; businesses.ErrNotFound if missing.
	DeleteBusiness func(ctx context.Context, businessID string) error
	Exports        ExportsRepo
	Imports        ImportsRepo
	Now            func() time.Time // injectable clock; nil = time.Now
}

type PlansRepo interface {
	ForUser(ctx context.Context, userID string) (plans.Account, bool, error)
	ForBusiness(ctx context.Context, businessID string) (plans.Account, error)
	OwnerOf(ctx context.Context, businessID string) (string, error)
	Set(ctx context.Context, userID string, a plans.Account, note string) error
	Usage(ctx context.Context, businessID, what, itemID string) (int, error)
	OwnedBusinesses(ctx context.Context, userID string) (int, error)
}

// businessPlan: what the business's owner's plan includes (a trial includes what its plan does).
func (d Deps) businessPlan(w http.ResponseWriter, r *http.Request) (plans.Account, bool) {
	m, _ := auth.MemberFrom(r.Context())
	a, err := d.Plans.ForBusiness(r.Context(), m.BusinessID)
	if err != nil {
		internalError(w, r, "account plan", err)
		return a, false
	}
	return a, true
}

// withinPlan: creating one more of `what` must fit the plan's limit (the itemId URL param counts photos).
func (d Deps) withinPlan(what string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, ok := d.businessPlan(w, r)
			if !ok {
				return
			}
			limit := plans.Get(a.Plan).Limit(what)
			if limit == nil {
				next.ServeHTTP(w, r)
				return
			}
			m, _ := auth.MemberFrom(r.Context())
			used, err := d.Plans.Usage(r.Context(), m.BusinessID, what, chi.URLParam(r, "itemId"))
			if err != nil {
				internalError(w, r, "plan usage", err)
				return
			}
			if !plans.HasRoom(limit, used) {
				apierr.Write(w, http.StatusForbidden, "plan_limit", plans.LimitMessage(a.Plan, what, *limit))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func planMissing(w http.ResponseWriter, a plans.Account) {
	apierr.Write(w, http.StatusForbidden, "plan_capability", "Esta función no está incluida en tu plan "+strings.ToUpper(a.Plan[:1])+a.Plan[1:]+".")
}

// requireCapability: a part of the app that only some plans include (sueldos, gastos recurrentes…).
func (d Deps) requireCapability(capability string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, ok := d.businessPlan(w, r)
			if !ok {
				return
			}
			if !plans.Get(a.Plan).Can(capability) {
				planMissing(w, a)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// planExpired: what an expired account gets when it tries to change something.
func planExpired(w http.ResponseWriter, a plans.Account) {
	msg := "Tu periodo de prueba terminó. Puedes consultar tu información; para seguir registrando, elige un plan."
	if a.Plan != plans.Trial {
		msg = "Tu plan venció. Puedes consultar tu información; para seguir registrando, renueva tu plan."
	}
	apierr.Write(w, http.StatusForbidden, "plan_expired", msg)
}

// requireWritable: a business whose owner's account expired is read-only (reading and exporting still work).
func (d Deps) requireWritable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		m, _ := auth.MemberFrom(r.Context())
		// Deleting the business is allowed even when expired: their data, their call.
		if r.Method == http.MethodDelete && strings.TrimSuffix(r.URL.Path, "/") == "/v1/b/"+m.BusinessID {
			next.ServeHTTP(w, r)
			return
		}
		a, err := d.Plans.ForBusiness(r.Context(), m.BusinessID)
		if err != nil {
			internalError(w, r, "account plan", err)
			return
		}
		if !a.Writable(d.now()) {
			planExpired(w, a)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// deleteBusiness: the business's name typed again confirms it (spaces and case don't matter).
func (d Deps) deleteBusiness(w http.ResponseWriter, r *http.Request, businessID string) {
	var body struct {
		Confirm string `json:"confirm"`
	}
	if !decode(w, r, &body) {
		return
	}
	b, err := d.Platform.Business(r.Context(), businessID)
	if errors.Is(err, platform.ErrNotFound) {
		notFound(w, "El negocio no existe.")
		return
	}
	if err != nil {
		internalError(w, r, "get business", err)
		return
	}
	if !strings.EqualFold(strings.Join(strings.Fields(body.Confirm), " "), strings.Join(strings.Fields(b.Name), " ")) {
		invalid(w, "Escribe el nombre del negocio para confirmar.", map[string]string{"confirm": "El nombre no coincide."})
		return
	}
	switch err := d.DeleteBusiness(r.Context(), businessID); {
	case errors.Is(err, businesses.ErrNotFound):
		notFound(w, "El negocio no existe.")
	case err != nil:
		internalError(w, r, "delete business", err)
	default:
		slog.InfoContext(r.Context(), "business deleted", "business", businessID, "name", b.Name)
		w.WriteHeader(http.StatusNoContent)
	}
}

type ExportsRepo interface {
	Timezone(ctx context.Context, businessID string) (string, error)
	Rows(ctx context.Context, businessID, kind string, p *exports.Period) (any, error)
}

type ImportsRepo interface {
	Customers(ctx context.Context, businessID string, rows []customers.Input, now time.Time, apply bool) (imports.Summary, error)
	Catalog(ctx context.Context, businessID string, rows []imports.CatalogRow, apply bool) (imports.Summary, error)
	Products(ctx context.Context, businessID string, rows []products.ProductInput, apply bool) (imports.Summary, error)
}

// importRows decodes {rows: [...]} with 1..MaxRows rows.
func importRows[T any](w http.ResponseWriter, r *http.Request) ([]T, bool) {
	var body struct {
		Rows []T `json:"rows"`
	}
	if !decodeMax(w, r, &body, 2<<20) { // a file of up to MaxRows rows
		return nil, false
	}
	if len(body.Rows) == 0 || len(body.Rows) > imports.MaxRows {
		msg := fmt.Sprintf("El archivo debe tener de 1 a %d filas.", imports.MaxRows)
		invalid(w, msg, map[string]string{"rows": msg})
		return nil, false
	}
	return body.Rows, true
}

type PlatformRepo interface {
	Businesses(ctx context.Context) ([]platform.Business, error)
	SetGrant(ctx context.Context, id, feature string, grant *bool) (platform.Business, error)
	Business(ctx context.Context, id string) (platform.Business, error)
	Verticals(ctx context.Context) ([]platform.Vertical, error)
	VerticalStatus(ctx context.Context, vertical string) (string, error)
	SetVerticalStatus(ctx context.Context, vertical, status string) (platform.Vertical, error)
}

type SettingsRepo interface {
	Get(ctx context.Context, businessID string) (settings.Settings, error)
	Save(ctx context.Context, businessID string, s settings.Settings) error
	DocumentTypes(ctx context.Context, businessID string) ([]settings.DocumentType, error)
	DocumentType(ctx context.Context, businessID, id string) (settings.DocumentType, error)
	CreateDocumentType(ctx context.Context, businessID string, d settings.DocumentType) (settings.DocumentType, error)
	UpdateDocumentType(ctx context.Context, businessID string, d settings.DocumentType) (settings.DocumentType, error)
	Notice(ctx context.Context, slug string) (settings.Notice, error)
}

type BookingsRepo interface {
	Create(ctx context.Context, businessID string, in bookings.CreateInput, now time.Time) (bookings.Booking, bool, error)
	Get(ctx context.Context, businessID, id string) (bookings.Booking, error)
	List(ctx context.Context, businessID string, f bookings.ListFilter) ([]bookings.Summary, error)
	Transition(ctx context.Context, businessID, id, to, actor, note string) (bookings.Booking, error)
	AddPayment(ctx context.Context, businessID, id string, p bookings.Payment, actor string, now time.Time, exists bookings.FileExists) (bookings.Booking, error)
	StepUpload(ctx context.Context, businessID, bookingID, stepID string) (string, error)
	CompleteStep(ctx context.Context, businessID, bookingID, stepID string, sub steps.Submission, actor string, now time.Time, exists bookings.FileExists) (bookings.Booking, error)
	Checkout(ctx context.Context, businessID, id, condition, actor string, now time.Time) (bookings.Booking, error)
	Return(ctx context.Context, businessID, id, condition, actor string, now time.Time) (bookings.Booking, error)
	Extend(ctx context.Context, businessID, id string, newEnd time.Time, actor string, now time.Time) (bookings.Booking, error)
	SetDiscount(ctx context.Context, businessID, id string, d bookings.Discount, actor string) (bookings.Booking, error)
	Inventory(ctx context.Context, businessID string) ([]bookings.InventoryRow, error)
	Quote(ctx context.Context, businessID string, in bookings.QuoteRequest) (bookings.QuoteResult, error)
	History(ctx context.Context, businessID string, f bookings.HistoryFilter) ([]bookings.Summary, error)
	RemoveStepFile(ctx context.Context, businessID, bookingID, stepID, key string) (bookings.Booking, error)
	AddDocument(ctx context.Context, businessID, bookingID, name string, keys []string, actor string, exists bookings.FileExists) (bookings.Booking, error)
	RemoveDocument(ctx context.Context, businessID, bookingID, docID string) (bookings.Booking, []string, error)
}

// writeBooking maps the bookings errors to HTTP and writes the booking on success.
func writeBooking(w http.ResponseWriter, r *http.Request, b bookings.Booking, err error) {
	var bad bookings.Invalid
	var steps bookings.StepsPending
	switch {
	case errors.As(err, &bad):
		invalid(w, bad.Message, map[string]string{bad.Field: bad.Message})
	case errors.As(err, &steps):
		apierr.JSON(w, http.StatusUnprocessableEntity, apierr.Error{Code: "steps_pending", Message: steps.Error(), Details: map[string]any{"steps": steps.Labels}})
	case errors.Is(err, bookings.ErrNotFound):
		notFound(w, "La reserva no existe.")
	case errors.Is(err, bookings.ErrSlotTaken):
		apierr.Write(w, http.StatusConflict, "slot_taken", "Una de las unidades ya está apartada para esas fechas.")
	case err != nil:
		internalError(w, r, "booking", err)
	default:
		apierr.JSON(w, http.StatusOK, b)
	}
}

// fileExists checks an uploaded object is really in the bucket (before referencing it).
func (d Deps) fileExists(ctx context.Context, bucket, key string) error {
	if d.Photos == nil || d.Photos.Files == nil {
		return errors.New("files not configured")
	}
	_, _, err := d.Photos.Files.Head(ctx, bucket, key)
	return err
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
	Delete(ctx context.Context, businessID, staffID string) error
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
func decode(w http.ResponseWriter, r *http.Request, v any) bool { return decodeMax(w, r, v, 16<<10) }

func decodeMax(w http.ResponseWriter, r *http.Request, v any, max int64) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, max)).Decode(v); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_json", "El cuerpo de la petición no es JSON válido.")
		return false
	}
	return true
}

func internalError(w http.ResponseWriter, r *http.Request, what string, err error) {
	slog.ErrorContext(r.Context(), what, "err", err)
	monitor.Capture(r.Context(), err, what)
	apierr.Write(w, http.StatusInternalServerError, "internal", "Error interno.")
}

func invalid(w http.ResponseWriter, message string, errs map[string]string) {
	apierr.JSON(w, http.StatusUnprocessableEntity, apierr.Error{Code: "validation_failed", Message: message, Details: errs})
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, monitor.Middleware) // monitor reports a panic, Recoverer answers 500
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

	// Public (no session): what a customer reads before giving their data to a business.
	r.Get("/v1/public/businesses/{slug}/privacy", func(w http.ResponseWriter, r *http.Request) {
		n, err := d.Settings.Notice(r.Context(), chi.URLParam(r, "slug"))
		switch {
		case errors.Is(err, settings.ErrNotFound):
			notFound(w, "El negocio no existe.")
		case err != nil:
			internalError(w, r, "privacy notice", err)
		default:
			apierr.JSON(w, http.StatusOK, n)
		}
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
			for i, m := range ms {
				var s struct {
					Features map[string]bool `json:"features"`
				}
				_ = json.Unmarshal(m.Settings, &s)
				ms[i].Features = features.Active(m.Vertical, features.ParseForPlan(m.Grants, m.Plan), s.Features)
				a, err := d.Plans.ForBusiness(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "account plan", err)
					return
				}
				v := a.View(d.now())
				ms[i].Account = &v
			}
			admin, err := d.Store.IsPlatformAdmin(r.Context(), user)
			if err != nil {
				internalError(w, r, "platform admin", err)
				return
			}
			apierr.JSON(w, http.StatusOK, map[string]any{"user_id": user, "businesses": ms, "platform_admin": admin})
		})

		// Platform admin: every business and its features (not tied to one business).
		r.Route("/admin", func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					user, _ := auth.UserID(r.Context())
					ok, err := d.Store.IsPlatformAdmin(r.Context(), user)
					if err != nil {
						internalError(w, r, "platform admin", err)
						return
					}
					if !ok {
						apierr.Write(w, http.StatusForbidden, "forbidden", "Solo el administrador de la plataforma.")
						return
					}
					next.ServeHTTP(w, r)
				})
			})
			r.Get("/verticals", func(w http.ResponseWriter, r *http.Request) {
				list, err := d.Platform.Verticals(r.Context())
				if err != nil {
					internalError(w, r, "admin verticals", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"verticals": list})
			})
			r.Put("/verticals/{vertical}", func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Status string `json:"status"`
				}
				if !decode(w, r, &body) {
					return
				}
				v, err := d.Platform.SetVerticalStatus(r.Context(), chi.URLParam(r, "vertical"), body.Status)
				switch {
				case errors.Is(err, platform.ErrUnknownVertical):
					notFound(w, "Ese giro no existe.")
				case errors.Is(err, platform.ErrBadStatus):
					invalid(w, "Elige público, solo superadministrador o apagado.", map[string]string{"status": "Estado inválido."})
				case err != nil:
					internalError(w, r, "set vertical", err)
				default:
					apierr.JSON(w, http.StatusOK, v)
				}
			})
			// Plan of the business owner's account (covers all their businesses). A date ends_at lasts
			// that whole day; a trial without one gets the usual length from now.
			r.With(requireUUIDParam("businessId", "El negocio no existe.")).Put("/businesses/{businessId}/plan", func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Plan   string `json:"plan"`
					EndsAt string `json:"ends_at"` // YYYY-MM-DD; empty = no end
					Note   string `json:"note"`
				}
				if !decode(w, r, &body) {
					return
				}
				if !plans.Valid(body.Plan) {
					invalid(w, "Elige un plan.", map[string]string{"plan": "Plan desconocido."})
					return
				}
				a := plans.Account{Plan: body.Plan}
				switch {
				case body.EndsAt != "":
					day, err := time.Parse("2006-01-02", body.EndsAt)
					if err != nil {
						invalid(w, "Revisa la fecha.", map[string]string{"ends_at": "Fecha inválida."})
						return
					}
					end := day.AddDate(0, 0, 1)
					a.EndsAt = &end
				case body.Plan == plans.Trial:
					a = plans.NewTrial(d.now())
				}
				if utf8.RuneCountInString(body.Note) > 200 {
					invalid(w, "La nota es muy larga.", map[string]string{"note": "Máximo 200 caracteres."})
					return
				}
				owner, err := d.Plans.OwnerOf(r.Context(), chi.URLParam(r, "businessId"))
				if errors.Is(err, plans.ErrNoOwner) {
					notFound(w, "El negocio no existe.")
					return
				}
				if err == nil {
					err = d.Plans.Set(r.Context(), owner, a, body.Note)
				}
				if err != nil {
					internalError(w, r, "set plan", err)
					return
				}
				apierr.JSON(w, http.StatusOK, a.View(d.now()))
			})
			r.With(requireUUIDParam("businessId", "El negocio no existe.")).Delete("/businesses/{businessId}", func(w http.ResponseWriter, r *http.Request) {
				d.deleteBusiness(w, r, chi.URLParam(r, "businessId"))
			})
			r.Get("/businesses", func(w http.ResponseWriter, r *http.Request) {
				list, err := d.Platform.Businesses(r.Context())
				if err != nil {
					internalError(w, r, "admin businesses", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"businesses": list})
			})
			r.With(requireUUIDParam("businessId", "El negocio no existe.")).Put("/businesses/{businessId}/features/{feature}", func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Grant *bool `json:"grant"` // true = give, false = remove, null = as the giro
				}
				if !decode(w, r, &body) {
					return
				}
				b, err := d.Platform.SetGrant(r.Context(), chi.URLParam(r, "businessId"), chi.URLParam(r, "feature"), body.Grant)
				switch {
				case errors.Is(err, platform.ErrNotFound):
					notFound(w, "El negocio no existe.")
				case errors.Is(err, platform.ErrNotForBusiness):
					invalid(w, "Esa función no aplica a este tipo de negocio.", map[string]string{"feature": "No aplica a este tipo de negocio."})
				case err != nil:
					internalError(w, r, "set grant", err)
				default:
					apierr.JSON(w, http.StatusOK, b)
				}
			})
		})

		// Giros this user can open a business of (platform_verticals; the platform admin also sees the ones being tried out).
		r.Get("/verticals", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.UserID(r.Context())
			admin, err := d.Store.IsPlatformAdmin(r.Context(), user)
			if err != nil {
				internalError(w, r, "platform admin", err)
				return
			}
			list, err := d.Platform.Verticals(r.Context())
			if err != nil {
				internalError(w, r, "verticals", err)
				return
			}
			apierr.JSON(w, http.StatusOK, map[string]any{"verticals": platform.AvailableFor(list, admin)})
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
			status, err := d.Platform.VerticalStatus(r.Context(), in.Vertical)
			if err != nil {
				internalError(w, r, "vertical status", err)
				return
			}
			admin, err := d.Store.IsPlatformAdmin(r.Context(), user)
			if err != nil {
				internalError(w, r, "platform admin", err)
				return
			}
			if !platform.CanCreate(status, admin) {
				invalid(w, "Ese giro todavía no está disponible.", map[string]string{"vertical": "Ese giro todavía no está disponible."})
				return
			}
			// An owner whose account expired can't open more businesses (a new owner gets a trial), nor
			// more than their plan allows; the platform admin tries giros out without that cap.
			acct, ok, err := d.Plans.ForUser(r.Context(), user)
			if err != nil {
				internalError(w, r, "account plan", err)
				return
			}
			if !ok {
				acct = plans.NewTrial(d.now())
			}
			if !acct.Writable(d.now()) {
				planExpired(w, acct)
				return
			}
			if limit := plans.Get(acct.Plan).Limit("businesses"); !admin {
				owned, err := d.Plans.OwnedBusinesses(r.Context(), user)
				if err != nil {
					internalError(w, r, "owned businesses", err)
					return
				}
				if !plans.HasRoom(limit, owned) {
					apierr.Write(w, http.StatusForbidden, "plan_limit", plans.LimitMessage(acct.Plan, "businesses", *limit))
					return
				}
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
			r.Use(auth.RequireMember(d.Store), d.requireWritable)
			r.With(auth.RequireRole("owner")).Delete("/", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				d.deleteBusiness(w, r, m.BusinessID)
			})

			// Everything of one kind, unpaginated, for the owner's CSV (also when the account expired:
			// their data is theirs). ?from=&to= (YYYY-MM-DD) narrows dated kinds; none = all.
			r.With(auth.RequireRole("owner")).Get("/export/{kind}", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				kind := chi.URLParam(r, "kind")
				if !exports.Valid(kind) {
					notFound(w, "No se puede exportar eso.")
					return
				}
				var p *exports.Period
				if q := r.URL.Query(); q.Get("from") != "" || q.Get("to") != "" {
					from, ok1 := finances.ParseDay(q.Get("from"))
					to, ok2 := finances.ParseDay(q.Get("to"))
					if !ok1 || !ok2 || to.Before(from) {
						invalid(w, "Indica un periodo válido.", map[string]string{"from": "Periodo inválido."})
						return
					}
					p = &exports.Period{From: from, To: to}
				}
				tz, err := d.Exports.Timezone(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "export timezone", err)
					return
				}
				rows, err := d.Exports.Rows(r.Context(), m.BusinessID, kind, p)
				if err != nil {
					internalError(w, r, "export "+kind, err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"timezone": tz, "rows": rows})
			})

			// Import rows the app parsed from a CSV: the plan row by row (create, update or error), and
			// with ?apply=true the valid rows are saved in one transaction.
			r.With(auth.RequireRole("owner")).Post("/import/{kind}", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				apply := r.URL.Query().Get("apply") == "true"
				var sum imports.Summary
				var err error
				switch chi.URLParam(r, "kind") {
				case "customers":
					rows, ok := importRows[customers.Input](w, r)
					if !ok {
						return
					}
					sum, err = d.Imports.Customers(r.Context(), m.BusinessID, rows, d.now(), apply)
				case "catalog":
					rows, ok := importRows[imports.CatalogRow](w, r)
					if !ok {
						return
					}
					sum, err = d.Imports.Catalog(r.Context(), m.BusinessID, rows, apply)
				case "products":
					rows, ok := importRows[products.ProductInput](w, r)
					if !ok {
						return
					}
					sum, err = d.Imports.Products(r.Context(), m.BusinessID, rows, apply)
				default:
					notFound(w, "No se puede importar eso.")
					return
				}
				if err != nil {
					internalError(w, r, "import", err)
					return
				}
				apierr.JSON(w, http.StatusOK, sum)
			})
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

			r.With(auth.RequireRole("owner"), d.withinPlan("catalog_items")).Post("/catalog", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				var body struct {
					catalog.Item
					Units int `json:"units"` // rentals: how many pieces to create right away (codes like ROT-01)
				}
				if !decode(w, r, &body) {
					return
				}
				if body.Units < 0 || body.Units > 50 {
					invalid(w, "Revisa las unidades.", map[string]string{"units": "Escribe de 0 a 50 piezas."})
					return
				}
				it := body.Item.Normalize()
				if errs := it.Validate(); len(errs) > 0 {
					invalid(w, "Revisa los datos del artículo.", errs)
					return
				}
				created, err := d.Catalog.Create(r.Context(), m.BusinessID, it)
				if err != nil {
					internalError(w, r, "create catalog item", err)
					return
				}
				for n := 1; catalog.HasUnits(created.Kind) && n <= body.Units; n++ {
					u := catalog.Unit{ItemID: created.ID, Code: catalog.UnitCode(created.Name, n), Status: "available"}
					if _, err := d.Catalog.CreateUnit(r.Context(), m.BusinessID, u); err != nil && !errors.Is(err, catalog.ErrCodeTaken) {
						internalError(w, r, "create item units", err)
						return
					}
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

			mountImages(r, d.Photos, d.withinPlan("photos_per_item"))
			mountCustomers(r, d)
			mountFinances(r, d)
			mountProducts(r, d)

			r.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				st, err := d.Settings.Get(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "get settings", err)
					return
				}
				apierr.JSON(w, http.StatusOK, st)
			})

			// Extra documents asked in every rental ("Documentos adicionales"), configured by the owner.
			r.Get("/settings/documents", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				list, err := d.Settings.DocumentTypes(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "document types", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"documents": list})
			})
			saveDocType := func(w http.ResponseWriter, r *http.Request, current settings.DocumentType, create bool) {
				m, _ := auth.MemberFrom(r.Context())
				var in settings.DocumentTypeInput
				if !decode(w, r, &in) {
					return
				}
				next := current.Apply(in)
				if errs := next.Validate(); errs != nil {
					invalid(w, "Revisa el documento.", errs)
					return
				}
				var saved settings.DocumentType
				var err error
				if create {
					saved, err = d.Settings.CreateDocumentType(r.Context(), m.BusinessID, next)
				} else {
					saved, err = d.Settings.UpdateDocumentType(r.Context(), m.BusinessID, next)
				}
				if err != nil {
					internalError(w, r, "save document type", err)
					return
				}
				apierr.JSON(w, map[bool]int{true: http.StatusCreated, false: http.StatusOK}[create], saved)
			}
			r.With(auth.RequireRole("owner")).Post("/settings/documents", func(w http.ResponseWriter, r *http.Request) {
				saveDocType(w, r, settings.DocumentType{Active: true}, true)
			})
			r.With(auth.RequireRole("owner"), requireUUIDParam("docTypeId", "El documento no existe.")).Patch("/settings/documents/{docTypeId}", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				current, err := d.Settings.DocumentType(r.Context(), m.BusinessID, chi.URLParam(r, "docTypeId"))
				if errors.Is(err, settings.ErrNotFound) {
					notFound(w, "El documento no existe.")
					return
				}
				if err != nil {
					internalError(w, r, "document type", err)
					return
				}
				saveDocType(w, r, current, false)
			})

			r.With(auth.RequireRole("owner")).Patch("/settings", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				var patch settings.Patch
				if !decode(w, r, &patch) {
					return
				}
				current, err := d.Settings.Get(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "get settings", err)
					return
				}
				next, errs := current.Apply(patch)
				if len(errs) > 0 {
					invalid(w, "Revisa los ajustes.", errs)
					return
				}
				if err := d.Settings.Save(r.Context(), m.BusinessID, next); err != nil {
					internalError(w, r, "save settings", err)
					return
				}
				apierr.JSON(w, http.StatusOK, next)
			})

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

			// Price of a booking before creating it (the form shows it while dates are picked).
			r.Get("/quote", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				q := r.URL.Query()
				in := bookings.QuoteRequest{ItemID: q.Get("item_id")}
				in.Quantity, _ = strconv.Atoi(q.Get("quantity"))
				start, err := time.Parse(time.RFC3339, q.Get("starts_at"))
				if err != nil || !auth.IsUUID(in.ItemID) || in.Quantity > 100 {
					invalid(w, "Revisa los datos.", map[string]string{"starts_at": "Fecha u artículo inválido."})
					return
				}
				in.StartsAt = start
				if v := q.Get("ends_at"); v != "" {
					end, err := time.Parse(time.RFC3339, v)
					if err != nil {
						invalid(w, "Fecha de regreso inválida.", map[string]string{"ends_at": "Fecha inválida."})
						return
					}
					in.EndsAt = &end
				}
				if v := q.Get("discount_value"); v != "" {
					in.Discount.Type = q.Get("discount_type")
					if in.Discount.Value, err = strconv.ParseInt(v, 10, 64); err != nil {
						invalid(w, "Descuento inválido.", map[string]string{"discount.value": "Descuento inválido."})
						return
					}
				}
				res, err := d.Bookings.Quote(r.Context(), m.BusinessID, in)
				var bad bookings.Invalid
				switch {
				case errors.As(err, &bad):
					invalid(w, bad.Message, map[string]string{bad.Field: bad.Message})
				case err != nil:
					internalError(w, r, "quote", err)
				default:
					apierr.JSON(w, http.StatusOK, res)
				}
			})

			// Rental inventory: every unit with its rentals out or to come, sorted by date.
			r.Get("/inventory", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				rows, err := d.Bookings.Inventory(r.Context(), m.BusinessID)
				if err != nil {
					internalError(w, r, "inventory", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"rows": rows})
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
				if kind != "" && !engines.Valid(kind) {
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

			// History: newest first, by group, search and customer; paged with before + before_id.
			r.Get("/bookings/history", func(w http.ResponseWriter, r *http.Request) {
				m, _ := auth.MemberFrom(r.Context())
				q := r.URL.Query()
				f := bookings.HistoryFilter{Group: q.Get("group"), Query: q.Get("q"), CustomerID: q.Get("customer_id"), BeforeID: q.Get("before_id")}
				f.Limit, _ = strconv.Atoi(q.Get("limit"))
				if f.Limit == 0 {
					f.Limit = 30
				}
				if !bookings.ValidHistoryGroup(f.Group) || (f.CustomerID != "" && !auth.IsUUID(f.CustomerID)) || len(f.Query) > 80 {
					invalid(w, "Filtro inválido.", map[string]string{"group": "Filtro inválido."})
					return
				}
				if v := q.Get("before"); v != "" {
					t, err := time.Parse(time.RFC3339Nano, v)
					if err != nil {
						invalid(w, "Cursor inválido.", map[string]string{"before": "Fecha inválida."})
						return
					}
					f.Before = &t
				}
				list, err := d.Bookings.History(r.Context(), m.BusinessID, f)
				if err != nil {
					internalError(w, r, "history", err)
					return
				}
				apierr.JSON(w, http.StatusOK, map[string]any{"bookings": list})
			})

			r.Route("/bookings/{bookingId}", func(r chi.Router) {
				r.Use(requireUUIDParam("bookingId", "La reserva no existe."))

				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					b, err := d.Bookings.Get(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"))
					writeBooking(w, r, b, err)
				})

				// Status change (owner or staff); the machine and the requirements are checked in bookings.
				r.Post("/transition", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						To   string `json:"to"`
						Note string `json:"note"`
					}
					if !decode(w, r, &body) {
						return
					}
					if len(body.Note) > 500 {
						invalid(w, "La nota es muy larga.", map[string]string{"note": "Máximo 500 caracteres."})
						return
					}
					b, err := d.Bookings.Transition(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), body.To, m.UserID, body.Note)
					writeBooking(w, r, b, err)
				})

				// Private files of the booking (ID, evidence): presigned upload for a step, then complete it.
				r.Post("/uploads", func(w http.ResponseWriter, r *http.Request) {
					if !d.Photos.enabled(w) {
						return
					}
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						StepID      string `json:"step_id"`
						Purpose     string `json:"purpose"` // "" (a step) | "payment" (evidence of a payment)
						ContentType string `json:"content_type"`
						Size        int64  `json:"size"`
					}
					if !decode(w, r, &body) {
						return
					}
					// Photos only, except private documents (free documents and private steps like an
					// ID or a promissory note), which also take PDF.
					check := func(document bool) bool {
						msg := media.ValidateUpload(body.ContentType, body.Size)
						if document {
							msg = media.ValidateDocument(body.ContentType, body.Size)
						}
						if msg != "" {
							invalid(w, msg, map[string]string{"file": msg})
						}
						return msg == ""
					}
					bookingID := chi.URLParam(r, "bookingId")
					if body.Purpose == "document" {
						if !check(true) {
							return
						}
						if _, err := d.Bookings.Get(r.Context(), m.BusinessID, bookingID); err != nil {
							writeBooking(w, r, bookings.Booking{}, err)
							return
						}
						key := bookings.DocumentFilePrefix(m.BusinessID, bookingID) + bookings.RandomName(media.DocumentExt(body.ContentType))
						url, err := d.Photos.Files.PresignPut(r.Context(), files.PrivateDocs, key, body.ContentType, body.Size)
						if err != nil {
							internalError(w, r, "presign", err)
							return
						}
						apierr.JSON(w, http.StatusCreated, map[string]any{"key": key, "upload_url": url,
							"upload_headers": map[string]string{"Content-Type": body.ContentType}})
						return
					}
					if body.Purpose == "payment" {
						if !check(false) {
							return
						}
						if _, err := d.Bookings.Get(r.Context(), m.BusinessID, bookingID); err != nil {
							writeBooking(w, r, bookings.Booking{}, err)
							return
						}
						key := bookings.PaymentFilePrefix(m.BusinessID, bookingID) + bookings.RandomName(media.Ext(body.ContentType))
						url, err := d.Photos.Files.PresignPut(r.Context(), files.PrivateDocs, key, body.ContentType, body.Size)
						if err != nil {
							internalError(w, r, "presign", err)
							return
						}
						apierr.JSON(w, http.StatusCreated, map[string]any{"key": key, "upload_url": url,
							"upload_headers": map[string]string{"Content-Type": body.ContentType}})
						return
					}
					if !auth.IsUUID(body.StepID) {
						notFound(w, "El paso no existe.")
						return
					}
					bucket, err := d.Bookings.StepUpload(r.Context(), m.BusinessID, bookingID, body.StepID)
					if errors.Is(err, bookings.ErrNotFound) {
						notFound(w, "El paso no existe o no lleva archivos.")
						return
					}
					if err != nil {
						internalError(w, r, "step upload", err)
						return
					}
					if !check(bucket == files.PrivateDocs) {
						return
					}
					key := bookings.NewFileKey(m.BusinessID, bookingID, media.DocumentExt(body.ContentType))
					url, err := d.Photos.Files.PresignPut(r.Context(), bucket, key, body.ContentType, body.Size)
					if err != nil {
						internalError(w, r, "presign", err)
						return
					}
					apierr.JSON(w, http.StatusCreated, map[string]any{"key": key, "upload_url": url,
						"upload_headers": map[string]string{"Content-Type": body.ContentType}})
				})

				r.With(requireUUIDParam("stepId", "El paso no existe.")).Post("/steps/{stepId}", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var sub steps.Submission
					if !decode(w, r, &sub) {
						return
					}
					if len(sub.Files) > 20 {
						invalid(w, "Demasiados archivos.", map[string]string{"files": "Máximo 20."})
						return
					}
					b, err := d.Bookings.CompleteStep(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), chi.URLParam(r, "stepId"), sub, m.UserID, d.now(), d.fileExists)
					writeBooking(w, r, b, err)
				})

				// Short-lived read URL for a private file of this booking.
				r.Get("/files", func(w http.ResponseWriter, r *http.Request) {
					if !d.Photos.enabled(w) {
						return
					}
					m, _ := auth.MemberFrom(r.Context())
					bucket, key := r.URL.Query().Get("bucket"), r.URL.Query().Get("key")
					if (bucket != files.PrivateDocs && bucket != files.RentalEvidence) ||
						!strings.HasPrefix(key, bookings.FilePrefix(m.BusinessID, chi.URLParam(r, "bookingId"))) || strings.Contains(key, "..") {
						notFound(w, "El archivo no existe.")
						return
					}
					url, err := d.Photos.Files.PresignGet(r.Context(), bucket, key)
					if err != nil {
						internalError(w, r, "presign get", err)
						return
					}
					apierr.JSON(w, http.StatusOK, map[string]string{"url": url})
				})

				// Remove one photo from a step (while the step can still change); the object goes too.
				r.With(requireUUIDParam("stepId", "El paso no existe.")).Delete("/steps/{stepId}/files", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					key := r.URL.Query().Get("key")
					b, err := d.Bookings.RemoveStepFile(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), chi.URLParam(r, "stepId"), key)
					if err == nil && d.Photos != nil && d.Photos.Files != nil {
						// best effort: the reference is gone either way; the step's bucket is one of the two private ones
						_ = d.Photos.Files.Delete(r.Context(), files.PrivateDocs, key)
						_ = d.Photos.Files.Delete(r.Context(), files.RentalEvidence, key)
					}
					writeBooking(w, r, b, err)
				})

				r.Post("/documents", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						Name  string   `json:"name"`
						Files []string `json:"files"`
					}
					if !decode(w, r, &body) {
						return
					}
					b, err := d.Bookings.AddDocument(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), body.Name, body.Files, m.UserID, d.fileExists)
					writeBooking(w, r, b, err)
				})

				r.With(requireUUIDParam("docId", "El documento no existe.")).Delete("/documents/{docId}", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					b, keys, err := d.Bookings.RemoveDocument(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), chi.URLParam(r, "docId"))
					if err == nil && d.Photos != nil && d.Photos.Files != nil {
						for _, k := range keys {
							_ = d.Photos.Files.Delete(r.Context(), files.PrivateDocs, k) // best effort: the row is gone
						}
					}
					writeBooking(w, r, b, err)
				})

				r.Post("/checkout", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						Condition string `json:"condition"`
					}
					if !decode(w, r, &body) {
						return
					}
					b, err := d.Bookings.Checkout(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), body.Condition, m.UserID, d.now())
					writeBooking(w, r, b, err)
				})

				r.Post("/extend", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						EndsAt *time.Time `json:"ends_at"`
					}
					if !decode(w, r, &body) {
						return
					}
					if body.EndsAt == nil {
						invalid(w, "Elige la nueva fecha de regreso.", map[string]string{"ends_at": "Elige la nueva fecha de regreso."})
						return
					}
					b, err := d.Bookings.Extend(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), *body.EndsAt, m.UserID, d.now())
					writeBooking(w, r, b, err)
				})

				r.Put("/discount", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body bookings.Discount
					if !decode(w, r, &body) {
						return
					}
					b, err := d.Bookings.SetDiscount(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), body, m.UserID)
					writeBooking(w, r, b, err)
				})

				r.Post("/return", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var body struct {
						Condition string `json:"condition"`
					}
					if !decode(w, r, &body) {
						return
					}
					b, err := d.Bookings.Return(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), body.Condition, m.UserID, d.now())
					writeBooking(w, r, b, err)
				})

				r.Post("/payments", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					var p bookings.Payment
					if !decode(w, r, &p) {
						return
					}
					if p.Notes != nil && len(*p.Notes) > 500 {
						invalid(w, "La nota es muy larga.", map[string]string{"notes": "Máximo 500 caracteres."})
						return
					}
					b, err := d.Bookings.AddPayment(r.Context(), m.BusinessID, chi.URLParam(r, "bookingId"), p, m.UserID, d.now(), d.fileExists)
					writeBooking(w, r, b, err)
				})
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

			r.With(auth.RequireRole("owner"), d.withinPlan("staff")).Post("/staff", func(w http.ResponseWriter, r *http.Request) {
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

				r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
					m, _ := auth.MemberFrom(r.Context())
					switch err := d.Staff.Delete(r.Context(), m.BusinessID, chi.URLParam(r, "staffId")); {
					case errors.Is(err, staff.ErrNotFound):
						notFound(w, "La persona no existe.")
					case errors.Is(err, staff.ErrOwnerStaff):
						apierr.Write(w, http.StatusConflict, "owner_staff", "No puedes borrar a la persona ligada a tu cuenta.")
					case err != nil:
						internalError(w, r, "delete staff", err)
					default:
						w.WriteHeader(http.StatusNoContent)
					}
				})

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

			r.With(auth.RequireRole("owner"), d.withinPlan("rental_units")).Post("/catalog/{itemId}/units", func(w http.ResponseWriter, r *http.Request) {
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
