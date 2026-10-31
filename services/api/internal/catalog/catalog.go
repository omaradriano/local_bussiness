// Package catalog manages a business's catalog items (services, products, rentals).
package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("catalog item not found")

// Item mirrors public.catalog_items. Money in centavos.
type Item struct {
	ID              string  `json:"id"`
	Kind            string  `json:"kind"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Price           int64   `json:"price"`
	Active          bool    `json:"active"`
	Sort            int     `json:"sort"`
	DurationMinutes *int    `json:"duration_minutes"`
	BufferMinutes   int     `json:"buffer_minutes"`
	PrepHours       *int    `json:"prep_hours"`
	DailyCapacity   *int    `json:"daily_capacity"`
	PriceUnit       *string `json:"price_unit"`
	SecurityDeposit *int64  `json:"security_deposit"`
	LateFeePerDay   *int64  `json:"late_fee_per_day"`
	CustomValues    Values  `json:"custom_values"` // custom field id -> value (validated by customfields.Clean)
	Images          any     `json:"images"`        // filled by the handler (media.Image list); never stored here
}

// Values of the owner's custom fields; never null in the database.
type Values map[string]any

func (v Values) orEmpty() Values {
	if v == nil {
		return Values{}
	}
	return v
}

// Patch: nil fields are left as they are. Kind cannot change.
type Patch struct {
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	Price           *int64  `json:"price"`
	Active          *bool   `json:"active"`
	Sort            *int    `json:"sort"`
	DurationMinutes *int    `json:"duration_minutes"`
	BufferMinutes   *int    `json:"buffer_minutes"`
	PrepHours       *int    `json:"prep_hours"`
	DailyCapacity   *int    `json:"daily_capacity"`
	PriceUnit       *string `json:"price_unit"`
	SecurityDeposit *int64  `json:"security_deposit"`
	LateFeePerDay   *int64  `json:"late_fee_per_day"`
	CustomValues    *Values `json:"custom_values"`
}

func (it Item) Apply(p Patch) Item {
	if p.CustomValues != nil {
		it.CustomValues = *p.CustomValues
	}
	if p.Name != nil {
		it.Name = *p.Name
	}
	if p.Description != nil {
		it.Description = *p.Description
	}
	if p.Price != nil {
		it.Price = *p.Price
	}
	if p.Active != nil {
		it.Active = *p.Active
	}
	if p.Sort != nil {
		it.Sort = *p.Sort
	}
	if p.DurationMinutes != nil {
		it.DurationMinutes = p.DurationMinutes
	}
	if p.BufferMinutes != nil {
		it.BufferMinutes = *p.BufferMinutes
	}
	if p.PrepHours != nil {
		it.PrepHours = p.PrepHours
	}
	if p.DailyCapacity != nil {
		it.DailyCapacity = p.DailyCapacity
	}
	if p.PriceUnit != nil {
		it.PriceUnit = p.PriceUnit
	}
	if p.SecurityDeposit != nil {
		it.SecurityDeposit = p.SecurityDeposit
	}
	if p.LateFeePerDay != nil {
		it.LateFeePerDay = p.LateFeePerDay
	}
	return it
}

// Normalize trims text and clears the fields that belong to other kinds.
func (it Item) Normalize() Item {
	it.Name = strings.TrimSpace(it.Name)
	it.Description = strings.TrimSpace(it.Description)
	own := rulesByKind[it.Kind].fields
	for _, g := range fieldGroups {
		if !slices.Contains(own, g) {
			g.clear(&it)
		}
	}
	return it
}

const maxPrice = 100_000_000 // $1,000,000.00

// Validate returns field -> message (es-MX); mirrors the catalog_items checks.
func (it Item) Validate() map[string]string {
	errs := map[string]string{}
	if !engines.Valid(it.Kind) {
		errs["kind"] = "Tipo de artículo inválido."
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(it.Name)); n == 0 || n > 120 {
		errs["name"] = "Escribe un nombre (máximo 120 caracteres)."
	}
	if it.Price < 0 || it.Price > maxPrice {
		errs["price"] = "El precio debe estar entre $0 y $1,000,000."
	}
	for _, g := range rulesByKind[it.Kind].fields {
		g.validate(it, errs)
	}
	return errs
}

// HasUnits: the engine's items are physical units with a code each (created with the item).
func HasUnits(kind string) bool { return rulesByKind[kind].units }

// Repo reads and writes items; every query is scoped to the business.
type Repo struct{ DB store.DB }

const columns = `id::text, kind, name, coalesce(description, ''), price, active, sort, duration_minutes, buffer_minutes,
	prep_hours, daily_capacity, price_unit, security_deposit, late_fee_per_day, custom_values`

func scan(row pgx.Row) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.Kind, &it.Name, &it.Description, &it.Price, &it.Active, &it.Sort, &it.DurationMinutes,
		&it.BufferMinutes, &it.PrepHours, &it.DailyCapacity, &it.PriceUnit, &it.SecurityDeposit, &it.LateFeePerDay, &it.CustomValues)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, ErrNotFound
	}
	return it, err
}

func (r Repo) List(ctx context.Context, businessID string) ([]Item, error) {
	rows, err := r.DB.Query(ctx, `select `+columns+` from public.catalog_items where business_id = $1 and deleted_at is null order by sort, name`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		it, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r Repo) Get(ctx context.Context, businessID, itemID string) (Item, error) {
	return scan(r.DB.QueryRow(ctx, `select `+columns+` from public.catalog_items where business_id = $1 and id = $2 and deleted_at is null`, businessID, itemID))
}

func (r Repo) Create(ctx context.Context, businessID string, it Item) (Item, error) {
	return scan(r.DB.QueryRow(ctx, `
		insert into public.catalog_items
		  (business_id, kind, name, description, price, active, sort, duration_minutes, buffer_minutes,
		   prep_hours, daily_capacity, price_unit, security_deposit, late_fee_per_day, custom_values)
		values ($1, $2, $3, nullif($4, ''), $5, true,
		        coalesce((select max(sort) + 1 from public.catalog_items where business_id = $1), 1),
		        $6, $7, $8, $9, $10, $11, $12, $13)
		returning `+columns,
		businessID, it.Kind, it.Name, it.Description, it.Price, it.DurationMinutes, it.BufferMinutes,
		it.PrepHours, it.DailyCapacity, it.PriceUnit, it.SecurityDeposit, it.LateFeePerDay, it.CustomValues.orEmpty()))
}

func (r Repo) Update(ctx context.Context, businessID string, it Item) (Item, error) {
	return scan(r.DB.QueryRow(ctx, `
		update public.catalog_items set
		  name = $3, description = nullif($4, ''), price = $5, active = $6, sort = $7, duration_minutes = $8,
		  buffer_minutes = $9, prep_hours = $10, daily_capacity = $11, price_unit = $12,
		  security_deposit = $13, late_fee_per_day = $14, custom_values = $15
		where business_id = $1 and id = $2 and deleted_at is null
		returning `+columns,
		businessID, it.ID, it.Name, it.Description, it.Price, it.Active, it.Sort, it.DurationMinutes,
		it.BufferMinutes, it.PrepHours, it.DailyCapacity, it.PriceUnit, it.SecurityDeposit, it.LateFeePerDay, it.CustomValues.orEmpty()))
}
