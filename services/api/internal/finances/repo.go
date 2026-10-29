package finances

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("not found")

// Categories of an expense (also of a purchase in installments).
var Categories = []string{"transport", "maintenance", "rent", "utilities", "supplies", "asset", "advertising", "taxes", "payroll", "other"}

// Invalid: field -> message (es-MX).
type Invalid map[string]string

func (e Invalid) Error() string { return "invalid" }

const maxAmount = 100_000_000_00 // $100,000,000.00

type Expense struct {
	ID          string    `json:"id"`
	SpentOn     Day       `json:"spent_on"`
	Category    string    `json:"category"`
	Amount      int64     `json:"amount"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func (e Expense) Validate() error {
	errs := Invalid{}
	if !slices.Contains(Categories, e.Category) {
		errs["category"] = "Elige una categoría."
	}
	if e.Amount <= 0 || e.Amount > maxAmount {
		errs["amount"] = "Escribe un monto mayor a cero."
	}
	if e.SpentOn.IsZero() {
		errs["spent_on"] = "Elige la fecha."
	}
	if e.Description != nil && utf8.RuneCountInString(*e.Description) > 200 {
		errs["description"] = "La descripción es muy larga."
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (p Plan) Validate() error {
	errs := Invalid{}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Description)); n == 0 || n > 120 {
		errs["description"] = "Escribe qué se compró."
	}
	if !slices.Contains(Categories, p.Category) {
		errs["category"] = "Elige una categoría."
	}
	if p.DownPayment < 0 || p.DownPayment > maxAmount {
		errs["down_payment"] = "El enganche no es válido."
	}
	if p.InstallmentAmount <= 0 || p.InstallmentAmount > maxAmount {
		errs["installment_amount"] = "Escribe el monto de cada mensualidad."
	}
	if p.Installments < 1 || p.Installments > 360 {
		errs["installments"] = "El número de mensualidades debe ser de 1 a 360."
	}
	if p.PurchasedOn.IsZero() || p.FirstDue.IsZero() || p.FirstDue.Before(p.PurchasedOn) {
		errs["first_due"] = "El primer pago no puede ser antes de la compra."
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Pay of one team member: fixed salary per period and/or a commission on what their services collect.
type Pay struct {
	StaffID           string  `json:"staff_id"`
	Name              string  `json:"name"`
	Active            bool    `json:"active"`
	Salary            int64   `json:"salary"`
	SalaryPeriod      string  `json:"salary_period"` // weekly | biweekly | monthly
	CommissionPercent float64 `json:"commission_percent"`
}

func (p Pay) Validate() error {
	errs := Invalid{}
	if p.Salary < 0 || p.Salary > maxAmount {
		errs["salary"] = "El sueldo no es válido."
	}
	if !slices.Contains([]string{"weekly", "biweekly", "monthly"}, p.SalaryPeriod) {
		errs["salary_period"] = "Elige cada cuándo se paga."
	}
	if p.CommissionPercent < 0 || p.CommissionPercent > 100 {
		errs["commission_percent"] = "La comisión debe ser de 0 a 100 %."
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type Repo struct{ DB store.DB }

func (r Repo) Expenses(ctx context.Context, businessID string, from, to Day) ([]Expense, error) {
	rows, err := r.DB.Query(ctx, `select id::text, spent_on, category, amount, description, created_at from public.expenses
		where business_id = $1 and spent_on between $2 and $3 order by spent_on desc, created_at desc`, businessID, from, to)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Expense])
	if out == nil {
		out = []Expense{}
	}
	return out, err
}

func (r Repo) AddExpense(ctx context.Context, businessID string, e Expense, actor string) (Expense, error) {
	err := r.DB.QueryRow(ctx, `insert into public.expenses (business_id, spent_on, category, amount, description, created_by)
		values ($1, $2, $3, $4, nullif(trim($5), ''), nullif($6, '')::uuid) returning id::text, created_at`,
		businessID, e.SpentOn, e.Category, e.Amount, deref(e.Description), actor).Scan(&e.ID, &e.CreatedAt)
	return e, err
}

func (r Repo) DeleteExpense(ctx context.Context, businessID, id string) error {
	tag, err := r.DB.Exec(ctx, `delete from public.expenses where business_id = $1 and id = $2`, businessID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

const planCols = `id::text, description, category, purchased_on, down_payment, installment_amount, installments, first_due`

func (r Repo) Plans(ctx context.Context, businessID string) ([]Plan, error) {
	rows, err := r.DB.Query(ctx, `select `+planCols+` from public.expense_plans where business_id = $1 order by purchased_on desc`, businessID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Plan])
	if out == nil {
		out = []Plan{}
	}
	return out, err
}

func (r Repo) AddPlan(ctx context.Context, businessID string, p Plan, actor string) (Plan, error) {
	p.Description = strings.TrimSpace(p.Description)
	err := r.DB.QueryRow(ctx, `insert into public.expense_plans
		(business_id, description, category, purchased_on, down_payment, installment_amount, installments, first_due, created_by)
		values ($1, $2, $3, $4, $5, $6, $7, $8, nullif($9, '')::uuid) returning id::text`,
		businessID, p.Description, p.Category, p.PurchasedOn, p.DownPayment, p.InstallmentAmount, p.Installments, p.FirstDue, actor).Scan(&p.ID)
	return p, err
}

func (r Repo) DeletePlan(ctx context.Context, businessID, id string) error {
	tag, err := r.DB.Exec(ctx, `delete from public.expense_plans where business_id = $1 and id = $2`, businessID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Pays lists every team member with their pay (defaults when not set).
func (r Repo) Pays(ctx context.Context, businessID string) ([]Pay, error) {
	rows, err := r.DB.Query(ctx, `select s.id::text, s.name, s.active, coalesce(p.salary, 0), coalesce(p.salary_period, 'monthly'),
		coalesce(p.commission_percent, 0)::float8
		from public.staff s left join public.staff_pay p on p.staff_id = s.id
		where s.business_id = $1 order by s.active desc, s.created_at`, businessID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Pay])
	if out == nil {
		out = []Pay{}
	}
	return out, err
}

func (r Repo) SetPay(ctx context.Context, businessID string, p Pay) (Pay, error) {
	tag, err := r.DB.Exec(ctx, `insert into public.staff_pay (staff_id, business_id, salary, salary_period, commission_percent)
		select s.id, s.business_id, $3, $4, $5 from public.staff s where s.business_id = $1 and s.id = $2
		on conflict (staff_id) do update set salary = excluded.salary, salary_period = excluded.salary_period,
		  commission_percent = excluded.commission_percent, updated_at = now()`,
		businessID, p.StaffID, p.Salary, p.SalaryPeriod, p.CommissionPercent)
	if err == nil && tag.RowsAffected() == 0 {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	all, err := r.Pays(ctx, businessID)
	if i := slices.IndexFunc(all, func(x Pay) bool { return x.StaffID == p.StaffID }); err == nil && i >= 0 {
		return all[i], nil
	}
	return p, err
}

// ---------------------------------------------------------------- summary

// Part of a total (a source of income or a category of expense).
type Part struct {
	Key    string `json:"key"`
	Amount int64  `json:"amount"`
}

type Side struct {
	Total int64  `json:"total"`
	Parts []Part `json:"parts"` // biggest first
}

type PlanStatus struct {
	Plan
	Paid      int    `json:"paid"`      // installments already due
	Remaining int64  `json:"remaining"` // still owed
	NextDue   *Day   `json:"next_due"`
	InPeriod  int64  `json:"in_period"` // what it costs in the period
	Status    string `json:"status"`    // active | finished
}

type PayLine struct {
	StaffID    string `json:"staff_id"`
	Name       string `json:"name"`
	Salary     int64  `json:"salary"`     // fixed pay earned in the period
	Commission int64  `json:"commission"` // percent of what their services collected
}

type Summary struct {
	From     Day          `json:"from"`
	To       Day          `json:"to"`
	Income   Side         `json:"income"`   // bookings (advances + balances), fees (late, damage)
	Expenses Side         `json:"expenses"` // by category; payroll and commissions included
	Profit   int64        `json:"profit"`
	Plans    []PlanStatus `json:"plans"`
	Payroll  []PayLine    `json:"payroll"`
}

func side(m map[string]int64) Side {
	s := Side{Parts: []Part{}}
	for k, v := range m {
		if v != 0 {
			s.Parts = append(s.Parts, Part{k, v})
			s.Total += v
		}
	}
	sort.Slice(s.Parts, func(i, j int) bool {
		if s.Parts[i].Amount != s.Parts[j].Amount {
			return s.Parts[i].Amount > s.Parts[j].Amount
		}
		return s.Parts[i].Key < s.Parts[j].Key
	})
	return s
}

// Summary of [from, to] (business dates, both included). Fixed pay counts only up to today (the
// business's local date at now): no future salary.
func (r Repo) Summary(ctx context.Context, businessID string, from, to Day, now time.Time) (Summary, error) {
	out := Summary{From: from, To: to, Plans: []PlanStatus{}, Payroll: []PayLine{}}
	var tz string
	if err := r.DB.QueryRow(ctx, `select timezone from public.businesses where id = $1`, businessID).Scan(&tz); err != nil {
		return out, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return out, err
	}
	ln := now.In(loc)
	today := time.Date(ln.Year(), ln.Month(), ln.Day(), 0, 0, 0, 0, time.UTC)
	// Payments are instants; the period is the business's local days.
	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)

	income := map[string]int64{}
	rows, err := r.DB.Query(ctx, `select case when type in ('late_fee', 'damage_fee') then 'fees' else 'bookings' end, sum(amount)
		from public.payments where business_id = $1 and paid_at >= $2 and paid_at < $3
		  and type in ('advance', 'balance', 'late_fee', 'damage_fee') group by 1`, businessID, start, end)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return out, err
		}
		income[k] += v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	// Product sales are income; merchandise bought for them is an expense («Mercancía»).
	var sold, bought int64
	if err := r.DB.QueryRow(ctx, `select coalesce(sum(total), 0) from public.sales where business_id = $1 and sold_at >= $2 and sold_at < $3`,
		businessID, start, end).Scan(&sold); err != nil {
		return out, err
	}
	income["products"] += sold
	if err := r.DB.QueryRow(ctx, `select coalesce(sum(quantity * unit_cost), 0) from public.stock_purchases
		where business_id = $1 and purchased_on between $2 and $3`, businessID, from, to).Scan(&bought); err != nil {
		return out, err
	}

	expenses := map[string]int64{"merchandise": bought}
	rows, err = r.DB.Query(ctx, `select category, sum(amount) from public.expenses
		where business_id = $1 and spent_on between $2 and $3 group by category`, businessID, from, to)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return out, err
		}
		expenses[k] += v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	plans, err := r.Plans(ctx, businessID)
	if err != nil {
		return out, err
	}
	for _, p := range plans {
		paid, remaining, next := p.Progress(today)
		ps := PlanStatus{Plan: p, Paid: paid, Remaining: remaining, NextDue: next, InPeriod: p.InRange(from, to), Status: "active"}
		if next == nil {
			ps.Status = "finished"
		}
		expenses[p.Category] += ps.InPeriod
		out.Plans = append(out.Plans, ps)
	}

	// Commission base: what was collected (advances + balances) for each person's bookings, split
	// evenly when a booking had more than one person.
	collected := map[string]float64{}
	rows, err = r.DB.Query(ctx, `
		select br.staff_id::text, sum(p.amount::float8 / (select count(*) from public.booking_resources x
		                                                  where x.booking_id = p.booking_id and x.staff_id is not null))
		from public.payments p join public.booking_resources br on br.booking_id = p.booking_id and br.staff_id is not null
		where p.business_id = $1 and p.paid_at >= $2 and p.paid_at < $3 and p.type in ('advance', 'balance')
		group by 1`, businessID, start, end)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return out, err
		}
		collected[k] = v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	pays, err := r.Pays(ctx, businessID)
	if err != nil {
		return out, err
	}
	for _, p := range pays {
		line := PayLine{StaffID: p.StaffID, Name: p.Name, Commission: Commission(collected[p.StaffID], p.CommissionPercent)}
		if p.Active {
			line.Salary = ProratedSalary(p.Salary, p.SalaryPeriod, from, to, today)
		}
		if line.Salary == 0 && line.Commission == 0 {
			continue
		}
		expenses["payroll"] += line.Salary
		expenses["commissions"] += line.Commission
		out.Payroll = append(out.Payroll, line)
	}

	out.Income, out.Expenses = side(income), side(expenses)
	out.Profit = out.Income.Total - out.Expenses.Total
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Today is the business's local date at now (for default periods).
func (r Repo) Today(ctx context.Context, businessID string, now time.Time) (Day, error) {
	var tz string
	if err := r.DB.QueryRow(ctx, `select timezone from public.businesses where id = $1`, businessID).Scan(&tz); err != nil {
		return Day{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Day{}, err
	}
	ln := now.In(loc)
	return time.Date(ln.Year(), ln.Month(), ln.Day(), 0, 0, 0, 0, time.UTC), nil
}
