package imports

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/phone"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

type Repo struct{ DB store.DB }

// inTx runs fn in one transaction: an import is all its valid rows, or nothing.
func (r Repo) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Customers plans the rows and, with apply, saves the valid ones.
func (r Repo) Customers(ctx context.Context, businessID string, rows []customers.Input, now time.Time, apply bool) (Summary, error) {
	cr := customers.Repo{DB: r.DB}
	rules, err := cr.Rules(ctx, businessID)
	if err != nil {
		return Summary{}, err
	}
	var phones []string
	for _, in := range rows {
		if in.Phone != nil {
			phones = append(phones, phone.Normalize(*in.Phone))
		}
	}
	existing := map[string]customers.Customer{}
	ids, err := r.DB.Query(ctx, `select id::text from public.customers where business_id = $1 and phone = any($2)`, businessID, phones)
	if err != nil {
		return Summary{}, err
	}
	found, err := pgx.CollectRows(ids, pgx.RowTo[string])
	if err != nil {
		return Summary{}, err
	}
	for _, id := range found {
		c, err := cr.Get(ctx, businessID, id)
		if err != nil {
			return Summary{}, err
		}
		existing[c.Phone] = c
	}
	planned := PlanCustomers(rows, existing, rules, now)
	results := make([]Result, len(planned))
	for i, p := range planned {
		results[i] = p.Result
	}
	s := summarize(results)
	if !apply || s.Created+s.Updated == 0 {
		return s, nil
	}
	err = r.inTx(ctx, func(tx pgx.Tx) error {
		repo := customers.Repo{DB: tx}
		for _, p := range planned {
			var err error
			switch p.Result.Action {
			case Create:
				_, err = repo.Create(ctx, businessID, p.Customer)
			case Update:
				_, err = repo.Update(ctx, businessID, p.Customer)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	s.Applied = err == nil
	return s, err
}

// businessPlan: the giro's engine (the kind new items get) and the plan's limit of catalog items.
func (r Repo) businessPlan(ctx context.Context, businessID string) (kind, plan string, err error) {
	var vertical string
	err = r.DB.QueryRow(ctx, `select vertical, plan from public.businesses where id = $1`, businessID).Scan(&vertical, &plan)
	return businesses.KindFor(vertical), plan, err
}

func (r Repo) Catalog(ctx context.Context, businessID string, rows []CatalogRow, apply bool) (Summary, error) {
	kind, plan, err := r.businessPlan(ctx, businessID)
	if err != nil {
		return Summary{}, err
	}
	existing, err := catalog.Repo{DB: r.DB}.List(ctx, businessID)
	if err != nil {
		return Summary{}, err
	}
	planned := PlanCatalog(rows, existing, kind, plans.Get(plan).Limit("catalog_items"), plan)
	results := make([]Result, len(planned))
	for i, p := range planned {
		results[i] = p.Result
	}
	s := summarize(results)
	if !apply || s.Created+s.Updated == 0 {
		return s, nil
	}
	err = r.inTx(ctx, func(tx pgx.Tx) error {
		repo := catalog.Repo{DB: tx}
		for _, p := range planned {
			var err error
			switch p.Result.Action {
			case Create:
				_, err = repo.Create(ctx, businessID, p.Item)
			case Update:
				_, err = repo.Update(ctx, businessID, p.Item)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	s.Applied = err == nil
	return s, err
}

func (r Repo) Products(ctx context.Context, businessID string, rows []products.ProductInput, apply bool) (Summary, error) {
	existing, err := products.Repo{DB: r.DB}.Products(ctx, businessID, true)
	if err != nil {
		return Summary{}, err
	}
	planned := PlanProducts(rows, existing)
	results := make([]Result, len(planned))
	for i, p := range planned {
		results[i] = p.Result
	}
	s := summarize(results)
	if !apply || s.Created+s.Updated == 0 {
		return s, nil
	}
	err = r.inTx(ctx, func(tx pgx.Tx) error {
		repo := products.Repo{DB: tx}
		for _, p := range planned {
			var err error
			switch p.Result.Action {
			case Create:
				_, err = repo.Create(ctx, businessID, p.Product)
			case Update:
				_, err = repo.Update(ctx, businessID, p.Product)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	s.Applied = err == nil
	return s, err
}
