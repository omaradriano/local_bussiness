// Package imports brings a business's existing lists (customers, catalog, products) from a CSV the
// app already parsed into rows. Every row goes through the same rules as the screens; the plan
// (create, update or fail, row by row) is shown first and applied only when confirmed.
package imports

import (
	"slices"
	"strings"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/phone"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
)

var Kinds = []string{"customers", "catalog", "products"}

func Valid(kind string) bool { return slices.Contains(Kinds, kind) }

const (
	Create = "create"
	Update = "update"
	Fail   = "error"

	MaxRows = 2000
)

// Result of one row; Row is its line in the file (1 is the header).
type Result struct {
	Row    int               `json:"row"`
	Name   string            `json:"name"`
	Action string            `json:"action"`
	Errors map[string]string `json:"errors,omitempty"`
}

type Summary struct {
	Results []Result `json:"results"`
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Failed  int      `json:"failed"`
	Applied bool     `json:"applied"`
}

func summarize(rs []Result) Summary {
	s := Summary{Results: rs}
	for _, r := range rs {
		switch r.Action {
		case Create:
			s.Created++
		case Update:
			s.Updated++
		default:
			s.Failed++
		}
	}
	return s
}

func result(i int, name string, errs map[string]string, update bool) Result {
	r := Result{Row: i + 2, Name: name, Action: Create}
	switch {
	case len(errs) > 0:
		r.Action, r.Errors = Fail, errs
	case update:
		r.Action = Update
	}
	return r
}

// ---------------------------------------------------------------- customers

type PlannedCustomer struct {
	Result   Result
	Customer customers.Customer
}

// PlanCustomers: a known phone updates that customer (only the columns that came), a new one creates
// it; a phone repeated in the file fails. existing is keyed by E.164 phone.
func PlanCustomers(rows []customers.Input, existing map[string]customers.Customer, rules customers.Rules, now time.Time) []PlannedCustomer {
	out := make([]PlannedCustomer, len(rows))
	seen := map[string]bool{}
	for i, in := range rows {
		cur, update := customers.Customer{}, false
		key := ""
		if in.Phone != nil {
			key = phone.Normalize(*in.Phone)
			cur, update = existing[key]
		}
		in.Trusted = nil // trust is decided by hand, not by a file
		c := cur.Apply(in, now)
		errs := c.Validate(rules)
		if key != "" && seen[key] && errs["phone"] == "" {
			errs["phone"] = "Este teléfono ya viene en otra fila del archivo."
		}
		seen[key] = true
		out[i] = PlannedCustomer{Result: result(i, c.Name, errs, update), Customer: c}
	}
	return out
}

// ---------------------------------------------------------------- catalog

// CatalogRow: the columns a catalog file brings (money in centavos; the kind is the giro's engine).
type CatalogRow struct {
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	Price           *int64  `json:"price"`
	DurationMinutes *int    `json:"duration_minutes"`
	PriceUnit       *string `json:"price_unit"`
	SecurityDeposit *int64  `json:"security_deposit"`
	LateFeePerDay   *int64  `json:"late_fee_per_day"`
	Active          *bool   `json:"active"`
}

type PlannedItem struct {
	Result Result
	Item   catalog.Item
}

func nameKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// PlanCatalog: an item with the same name (any case) is updated; new ones are created with the giro's
// kind, up to the plan's limit of active items (limit nil = none).
func PlanCatalog(rows []CatalogRow, existing []catalog.Item, kind string, limit *int, plan string) []PlannedItem {
	byName := map[string]catalog.Item{}
	active := 0
	for _, it := range existing {
		if it.Kind == kind {
			byName[nameKey(it.Name)] = it
		}
		if it.Active {
			active++
		}
	}
	out := make([]PlannedItem, len(rows))
	seen := map[string]bool{}
	for i, row := range rows {
		cur, update := byName[nameKey(row.Name)]
		if !update {
			cur = catalog.Item{Kind: kind, Active: true, Price: -1} // a new item needs its price
		}
		name := strings.Join(strings.Fields(row.Name), " ")
		it := cur.Apply(catalog.Patch{Name: &name, Description: row.Description, Price: row.Price, DurationMinutes: row.DurationMinutes,
			PriceUnit: row.PriceUnit, SecurityDeposit: row.SecurityDeposit, LateFeePerDay: row.LateFeePerDay, Active: row.Active}).Normalize()
		errs := it.Validate()
		if it.Price < 0 {
			errs["price"] = "Escribe el precio."
		}
		if k := nameKey(name); seen[k] && k != "" {
			errs["name"] = "Este nombre ya viene en otra fila del archivo."
		} else {
			seen[k] = true
		}
		if len(errs) == 0 && !update && it.Active {
			if !plans.HasRoom(limit, active) {
				errs["plan"] = plans.LimitMessage(plan, "catalog_items", *limit)
			} else {
				active++
			}
		}
		out[i] = PlannedItem{Result: result(i, it.Name, errs, update), Item: it}
	}
	return out
}

// ---------------------------------------------------------------- products

type PlannedProduct struct {
	Result  Result
	Product products.Product
}

// PlanProducts: same name (any case) updates price, cost and status; the stock of an existing product
// isn't overwritten (it moves with sales and purchases). New products start active with their stock.
func PlanProducts(rows []products.ProductInput, existing []products.Product) []PlannedProduct {
	byName := map[string]products.Product{}
	for _, p := range existing {
		byName[nameKey(p.Name)] = p
	}
	out := make([]PlannedProduct, len(rows))
	seen := map[string]bool{}
	for i, in := range rows {
		key := ""
		if in.Name != nil {
			key = nameKey(*in.Name)
		}
		cur, update := byName[key]
		if !update {
			cur = products.Product{Active: true}
		}
		p := cur.Apply(in, !update)
		errs := map[string]string{}
		if err := p.Validate(); err != nil {
			if bad, ok := err.(products.Invalid); ok {
				errs = bad
			}
		}
		if seen[key] && key != "" {
			errs["name"] = "Este nombre ya viene en otra fila del archivo."
		}
		seen[key] = true
		out[i] = PlannedProduct{Result: result(i, p.Name, errs, update), Product: p}
	}
	return out
}
