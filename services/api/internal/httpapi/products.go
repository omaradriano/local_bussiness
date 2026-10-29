package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/features"
	"github.com/omaradriano/local_bussiness/services/api/internal/finances"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
)

type ProductsRepo interface {
	Products(ctx context.Context, businessID string, withCost bool) ([]products.Product, error)
	Product(ctx context.Context, businessID, id string) (products.Product, error)
	Create(ctx context.Context, businessID string, p products.Product) (products.Product, error)
	Update(ctx context.Context, businessID string, p products.Product) (products.Product, error)
	Purchase(ctx context.Context, businessID, productID string, quantity int, unitCost int64, on time.Time, actor string) (products.Product, error)
	Sell(ctx context.Context, businessID string, in []products.LineInput, method, bookingID, actor string, now time.Time) (products.Sale, error)
	CancelSale(ctx context.Context, businessID, saleID string) error
	Sales(ctx context.Context, businessID string, start, end time.Time) ([]products.Sale, error)
	StatsFor(ctx context.Context, businessID string, start, end, from, to time.Time) (products.Stats, error)
	Bounds(ctx context.Context, businessID string, from, to time.Time) (time.Time, time.Time, error)
}

func writeProducts(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	var bad products.Invalid
	switch {
	case errors.As(err, &bad):
		invalid(w, "Revisa los datos.", bad)
	case errors.Is(err, products.ErrNotFound):
		notFound(w, "No existe.")
	case err != nil:
		internalError(w, r, "products", err)
	case v == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		apierr.JSON(w, status, v)
	}
}

// requireFeature: the route only works when the business has the feature active.
func (d Deps) requireFeature(feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			st, err := d.Settings.Get(r.Context(), m.BusinessID)
			if err != nil {
				internalError(w, r, "features", err)
				return
			}
			if !slices.ContainsFunc(st.Features, func(f features.State) bool { return f.Key == feature && f.Active }) {
				apierr.Write(w, http.StatusForbidden, "feature_off", "Tu negocio no tiene esta función activa.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// mountProducts: products for sale, sales, merchandise purchases and their stats (feature productSales).
func mountProducts(r chi.Router, d Deps) {
	r.Group(func(r chi.Router) {
		r.Use(d.requireFeature("productSales"))

		r.Get("/products", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			list, err := d.Products.Products(r.Context(), m.BusinessID, m.Role == "owner")
			writeProducts(w, r, http.StatusOK, map[string]any{"products": list}, err)
		})
		r.With(auth.RequireRole("owner")).Post("/products", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var in products.ProductInput
			if !decode(w, r, &in) {
				return
			}
			p := products.Product{Active: true}.Apply(in, true)
			if err := p.Validate(); err != nil {
				writeProducts(w, r, 0, nil, err)
				return
			}
			saved, err := d.Products.Create(r.Context(), m.BusinessID, p)
			writeProducts(w, r, http.StatusCreated, saved, err)
		})
		r.With(auth.RequireRole("owner"), requireUUIDParam("productId", "El producto no existe.")).Patch("/products/{productId}", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var in products.ProductInput
			if !decode(w, r, &in) {
				return
			}
			current, err := d.Products.Product(r.Context(), m.BusinessID, chi.URLParam(r, "productId"))
			if err != nil {
				writeProducts(w, r, 0, nil, err)
				return
			}
			p := current.Apply(in, false)
			if err := p.Validate(); err != nil {
				writeProducts(w, r, 0, nil, err)
				return
			}
			saved, err := d.Products.Update(r.Context(), m.BusinessID, p)
			writeProducts(w, r, http.StatusOK, saved, err)
		})
		r.With(auth.RequireRole("owner"), requireUUIDParam("productId", "El producto no existe.")).Post("/products/{productId}/purchases", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				Quantity    int    `json:"quantity"`
				UnitCost    int64  `json:"unit_cost"`
				PurchasedOn string `json:"purchased_on"`
			}
			if !decode(w, r, &body) {
				return
			}
			on, ok := finances.ParseDay(body.PurchasedOn)
			if !ok {
				invalid(w, "Elige la fecha de compra.", map[string]string{"purchased_on": "Fecha inválida."})
				return
			}
			p, err := d.Products.Purchase(r.Context(), m.BusinessID, chi.URLParam(r, "productId"), body.Quantity, body.UnitCost, on, m.UserID)
			writeProducts(w, r, http.StatusOK, p, err)
		})
		r.With(auth.RequireRole("owner")).Get("/products/stats", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			from, to, ok := d.period(w, r)
			if !ok {
				return
			}
			start, end, err := d.Products.Bounds(r.Context(), m.BusinessID, from, to)
			if err != nil {
				internalError(w, r, "bounds", err)
				return
			}
			st, err := d.Products.StatsFor(r.Context(), m.BusinessID, start, end, from, to)
			writeProducts(w, r, http.StatusOK, st, err)
		})

		// Anyone on the team sells at the counter; only the owner cancels a sale.
		r.Post("/sales", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				Lines     []products.LineInput `json:"lines"`
				Method    string               `json:"method"`
				BookingID string               `json:"booking_id"`
			}
			if !decode(w, r, &body) {
				return
			}
			if body.BookingID != "" && !auth.IsUUID(body.BookingID) {
				invalid(w, "La reserva no existe.", map[string]string{"booking_id": "Reserva inválida."})
				return
			}
			sale, err := d.Products.Sell(r.Context(), m.BusinessID, body.Lines, body.Method, body.BookingID, m.UserID, d.now())
			writeProducts(w, r, http.StatusCreated, sale, err)
		})
		r.Get("/sales", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			from, to, ok := d.period(w, r)
			if !ok {
				return
			}
			start, end, err := d.Products.Bounds(r.Context(), m.BusinessID, from, to)
			if err != nil {
				internalError(w, r, "bounds", err)
				return
			}
			list, err := d.Products.Sales(r.Context(), m.BusinessID, start, end)
			writeProducts(w, r, http.StatusOK, map[string]any{"sales": list}, err)
		})
		r.With(auth.RequireRole("owner"), requireUUIDParam("saleId", "La venta no existe.")).Delete("/sales/{saleId}", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			writeProducts(w, r, 0, nil, d.Products.CancelSale(r.Context(), m.BusinessID, chi.URLParam(r, "saleId")))
		})
	})
}
