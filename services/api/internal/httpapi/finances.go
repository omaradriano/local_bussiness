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
	Recurring(ctx context.Context, businessID string, today finances.Day) ([]finances.RecurringStatus, error)
	GetRecurring(ctx context.Context, businessID, id string) (finances.Recurring, error)
	AddRecurring(ctx context.Context, businessID string, r finances.Recurring, actor string) (finances.Recurring, error)
	DeleteRecurring(ctx context.Context, businessID, id string) error
	PayCharge(ctx context.Context, businessID string, r finances.Recurring, due, spentOn finances.Day, amount int64, actor string) (finances.Expense, error)
	SkipCharge(ctx context.Context, businessID string, r finances.Recurring, due finances.Day) error
}

func writeFinance(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	var bad finances.Invalid
	switch {
	case errors.As(err, &bad):
		invalid(w, "Revisa los datos.", bad)
	case errors.Is(err, finances.ErrNotFound):
		notFound(w, "No existe.")
	case errors.Is(err, finances.ErrAnswered):
		apierr.Write(w, http.StatusConflict, "already_answered", "Ese cobro ya se registró.")
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

		mountRecurring(r, d)

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

// mountRecurring: recurring expenses and the answer to each charge (paid with its real amount, or not paid).
func mountRecurring(r chi.Router, d Deps) {
	today := func(w http.ResponseWriter, r *http.Request) (finances.Day, bool) {
		m, _ := auth.MemberFrom(r.Context())
		day, err := d.Finances.Today(r.Context(), m.BusinessID, d.now())
		if err != nil {
			internalError(w, r, "today", err)
			return day, false
		}
		return day, true
	}

	r.Get("/recurring", func(w http.ResponseWriter, r *http.Request) {
		m, _ := auth.MemberFrom(r.Context())
		if day, ok := today(w, r); ok {
			list, err := d.Finances.Recurring(r.Context(), m.BusinessID, day)
			writeFinance(w, r, http.StatusOK, map[string]any{"recurring": list}, err)
		}
	})
	r.Post("/recurring", func(w http.ResponseWriter, r *http.Request) {
		m, _ := auth.MemberFrom(r.Context())
		var body struct {
			Description string `json:"description"`
			Category    string `json:"category"`
			Amount      int64  `json:"amount"`
			Frequency   string `json:"frequency"`
			StartsOn    string `json:"starts_on"`
			EndsOn      string `json:"ends_on"`
		}
		if !decode(w, r, &body) {
			return
		}
		start, _ := finances.ParseDay(body.StartsOn)
		rec := finances.Recurring{Description: body.Description, Category: body.Category, Amount: body.Amount, Frequency: body.Frequency, StartsOn: start}
		if body.EndsOn != "" {
			end, ok := finances.ParseDay(body.EndsOn)
			if !ok {
				invalid(w, "Revisa los datos.", map[string]string{"ends_on": "Fecha inválida."})
				return
			}
			rec.EndsOn = &end
		}
		if err := rec.Validate(); err != nil {
			writeFinance(w, r, 0, nil, err)
			return
		}
		saved, err := d.Finances.AddRecurring(r.Context(), m.BusinessID, rec, m.UserID)
		writeFinance(w, r, http.StatusCreated, saved, err)
	})

	r.Route("/recurring/{recurringId}", func(r chi.Router) {
		r.Use(requireUUIDParam("recurringId", "El gasto recurrente no existe."))
		r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			writeFinance(w, r, 0, nil, d.Finances.DeleteRecurring(r.Context(), m.BusinessID, chi.URLParam(r, "recurringId")))
		})

		// charge reads the recurring expense and checks due_on is one of its charges, up to the next one
		// (paying ahead is fine; further than that is a mistake).
		charge := func(w http.ResponseWriter, r *http.Request, dueOn string) (finances.Recurring, finances.Day, bool) {
			m, _ := auth.MemberFrom(r.Context())
			rec, err := d.Finances.GetRecurring(r.Context(), m.BusinessID, chi.URLParam(r, "recurringId"))
			if err != nil {
				writeFinance(w, r, 0, nil, err)
				return rec, finances.Day{}, false
			}
			day, ok := today(w, r)
			if !ok {
				return rec, day, false
			}
			due, ok := finances.ParseDay(dueOn)
			next := rec.NextDue(day)
			if !ok || !rec.IsDue(due) || (due.After(day) && (next == nil || due.After(*next))) {
				invalid(w, "Ese día no hay cobro de este gasto.", map[string]string{"due_on": "Ese día no hay cobro de este gasto."})
				return rec, due, false
			}
			return rec, due, true
		}
		r.Post("/charges", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				DueOn   string `json:"due_on"`
				Amount  int64  `json:"amount"`
				SpentOn string `json:"spent_on"` // when it was paid; the charge's date by default
			}
			if !decode(w, r, &body) {
				return
			}
			rec, due, ok := charge(w, r, body.DueOn)
			if !ok {
				return
			}
			spent := due
			if body.SpentOn != "" {
				if spent, ok = finances.ParseDay(body.SpentOn); !ok {
					invalid(w, "Revisa los datos.", map[string]string{"spent_on": "Fecha inválida."})
					return
				}
			}
			// Same rules as any expense (amount, date).
			if err := (finances.Expense{SpentOn: spent, Category: rec.Category, Amount: body.Amount}).Validate(); err != nil {
				writeFinance(w, r, 0, nil, err)
				return
			}
			saved, err := d.Finances.PayCharge(r.Context(), m.BusinessID, rec, due, spent, body.Amount, m.UserID)
			writeFinance(w, r, http.StatusCreated, saved, err)
		})
		r.Post("/skips", func(w http.ResponseWriter, r *http.Request) {
			m, _ := auth.MemberFrom(r.Context())
			var body struct {
				DueOn string `json:"due_on"`
			}
			if !decode(w, r, &body) {
				return
			}
			if rec, due, ok := charge(w, r, body.DueOn); ok {
				writeFinance(w, r, 0, nil, d.Finances.SkipCharge(r.Context(), m.BusinessID, rec, due))
			}
		})
	})
}
