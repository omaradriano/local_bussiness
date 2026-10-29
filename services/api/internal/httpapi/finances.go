package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/apierr"
	"github.com/omaradriano/local_bussiness/services/api/internal/auth"
	"github.com/omaradriano/local_bussiness/services/api/internal/finances"
)

type FinancesRepo interface {
	Today(ctx context.Context, businessID string, now time.Time) (finances.Day, error)
	Summary(ctx context.Context, businessID string, from, to finances.Day, now time.Time) (finances.Summary, error)
	Expenses(ctx context.Context, businessID string, from, to finances.Day) ([]finances.Expense, error)
	AddExpense(ctx context.Context, businessID string, e finances.Expense, actor string) (finances.Expense, error)
	DeleteExpense(ctx context.Context, businessID, id string) error
	Plans(ctx context.Context, businessID string) ([]finances.Plan, error)
	AddPlan(ctx context.Context, businessID string, p finances.Plan, actor string) (finances.Plan, error)
	DeletePlan(ctx context.Context, businessID, id string) error
	Pays(ctx context.Context, businessID string) ([]finances.Pay, error)
	SetPay(ctx context.Context, businessID string, p finances.Pay) (finances.Pay, error)
}

func writeFinance(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	var bad finances.Invalid
	switch {
	case errors.As(err, &bad):
		invalid(w, "Revisa los datos.", bad)
	case errors.Is(err, finances.ErrNotFound):
		notFound(w, "No existe.")
	case err != nil:
		internalError(w, r, "finances", err)
	case v == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		apierr.JSON(w, status, v)
	}
}

// period reads ?from=&to= (YYYY-MM-DD); by default the current month up to its last day.
func (d Deps) period(w http.ResponseWriter, r *http.Request) (finances.Day, finances.Day, bool) {
	m, _ := auth.MemberFrom(r.Context())
	q := r.URL.Query()
	if q.Get("from") == "" && q.Get("to") == "" {
		today, err := d.Finances.Today(r.Context(), m.BusinessID, d.now())
		if err != nil {
			internalError(w, r, "today", err)
			return today, today, false
		}
		first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return first, first.AddDate(0, 1, -1), true
	}
	from, ok1 := finances.ParseDay(q.Get("from"))
	to, ok2 := finances.ParseDay(q.Get("to"))
	if !ok1 || !ok2 || to.Before(from) || to.Sub(from) > 3*366*24*time.Hour {
		invalid(w, "Indica un periodo válido (hasta 3 años).", map[string]string{"from": "Periodo inválido."})
		return from, to, false
	}
	return from, to, true
}

// mountFinances: the owner's money (expenses, purchases in installments, the team's pay, summary).
func mountFinances(r chi.Router, d Deps) {
	r.Route("/finances", func(r chi.Router) {
		r.Use(auth.RequireRole("owner"))

		r.Get("/summary", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			if from, to, ok := d.period(w, r); ok {
				s, err := d.Finances.Summary(r.Context(), m.BusinessID, from, to, d.now())
				writeFinance(w, r, http.StatusOK, s, err)
			}
		})

		r.Get("/expenses", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			if from, to, ok := d.period(w, r); ok {
				list, err := d.Finances.Expenses(r.Context(), m.BusinessID, from, to)
				writeFinance(w, r, http.StatusOK, map[string]any{"expenses": list}, err)
			}
		})
		r.Post("/expenses", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				SpentOn     string  `json:"spent_on"`
				Category    string  `json:"category"`
				Amount      int64   `json:"amount"`
				Description *string `json:"description"`
			}
			if !decode(w, r, &body) {
				return
			}
			day, _ := finances.ParseDay(body.SpentOn)
			e := finances.Expense{SpentOn: day, Category: body.Category, Amount: body.Amount, Description: body.Description}
			if err := e.Validate(); err != nil {
				writeFinance(w, r, 0, nil, err)
				return
			}
			saved, err := d.Finances.AddExpense(r.Context(), m.BusinessID, e, m.UserID)
			writeFinance(w, r, http.StatusCreated, saved, err)
		})
		r.With(requireUUIDParam("expenseId", "El gasto no existe.")).Delete("/expenses/{expenseId}", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			writeFinance(w, r, 0, nil, d.Finances.DeleteExpense(r.Context(), m.BusinessID, chi.URLParam(r, "expenseId")))
		})

		r.Get("/plans", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			list, err := d.Finances.Plans(r.Context(), m.BusinessID)
			writeFinance(w, r, http.StatusOK, map[string]any{"plans": list}, err)
		})
		r.Post("/plans", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				Description       string `json:"description"`
				Category          string `json:"category"`
				PurchasedOn       string `json:"purchased_on"`
				DownPayment       int64  `json:"down_payment"`
				InstallmentAmount int64  `json:"installment_amount"`
				Installments      int    `json:"installments"`
				FirstDue          string `json:"first_due"`
			}
			if !decode(w, r, &body) {
				return
			}
			bought, _ := finances.ParseDay(body.PurchasedOn)
			first, _ := finances.ParseDay(body.FirstDue)
			p := finances.Plan{Description: body.Description, Category: body.Category, PurchasedOn: bought, DownPayment: body.DownPayment,
				InstallmentAmount: body.InstallmentAmount, Installments: body.Installments, FirstDue: first}
			if err := p.Validate(); err != nil {
				writeFinance(w, r, 0, nil, err)
				return
			}
			saved, err := d.Finances.AddPlan(r.Context(), m.BusinessID, p, m.UserID)
			writeFinance(w, r, http.StatusCreated, saved, err)
		})
		r.With(requireUUIDParam("planId", "La compra no existe.")).Delete("/plans/{planId}", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			writeFinance(w, r, 0, nil, d.Finances.DeletePlan(r.Context(), m.BusinessID, chi.URLParam(r, "planId")))
		})

		r.Get("/pay", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			list, err := d.Finances.Pays(r.Context(), m.BusinessID)
			writeFinance(w, r, http.StatusOK, map[string]any{"pay": list}, err)
		})
		r.With(requireUUIDParam("staffId", "La persona no existe.")).Put("/pay/{staffId}", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var p finances.Pay
			if !decode(w, r, &p) {
				return
			}
			p.StaffID = chi.URLParam(r, "staffId")
			if err := p.Validate(); err != nil {
				writeFinance(w, r, 0, nil, err)
				return
			}
			saved, err := d.Finances.SetPay(r.Context(), m.BusinessID, p)
			writeFinance(w, r, http.StatusOK, saved, err)
		})
	})
}
