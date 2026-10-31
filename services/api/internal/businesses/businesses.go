// Package businesses registers a business and applies its vertical's starting template.
package businesses

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/plans"
)

// Giros: each one has a template in templates/<giro>.json with its engine (kind), the features the
// API enforces and its starting catalog, hours, steps and settings. It's the API side of the giro map
// in packages/shared/src/verticals (a TS test checks both say the same). Adding a giro = a template here.
var (
	templates = loadTemplates()
	Verticals = slices.Sorted(maps.Keys(templates))
)

func loadTemplates() map[string]Template {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		panic(err)
	}
	out := map[string]Template{}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		raw, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			panic(err)
		}
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			panic(fmt.Sprintf("template %s: %v", name, err))
		}
		out[name] = t
	}
	return out
}

// KindFor is the engine (catalog item kind) a giro books with; "" for an unknown giro.
func KindFor(vertical string) string { return templates[vertical].Kind }

// HasFeature reports whether a giro brings a feature turned on (e.g. "guarantor").
func HasFeature(vertical, feature string) bool {
	return slices.Contains(templates[vertical].Features, feature)
}

// OffersFeature reports whether a giro offers a feature that starts off (the owner turns it on).
func OffersFeature(vertical, feature string) bool {
	return slices.Contains(templates[vertical].OptionalFeatures, feature)
}

// Slugs that would collide with app routes, since the public catalog lives at /<slug>.
var reserved = []string{"admin", "login", "signup", "onboarding", "api", "app", "assets", "static", "www", "help", "soporte", "privacidad", "terminos", "reset", "_expo"}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var ErrSlugTaken = errors.New("slug taken")

type Input struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Vertical string `json:"vertical"`
	Timezone string `json:"timezone,omitempty"`
}

func (in Input) Normalize() Input {
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.TrimSpace(in.Slug)
	if in.Timezone == "" {
		in.Timezone = "America/Chihuahua"
	}
	return in
}

// Validate returns field -> message (es-MX); empty when valid. Mirrors the DB checks.
func (in Input) Validate() map[string]string {
	errs := map[string]string{}
	if n := utf8.RuneCountInString(strings.TrimSpace(in.Name)); n == 0 || n > 80 {
		errs["name"] = "Escribe el nombre del negocio (máximo 80 caracteres)."
	}
	switch {
	case len(in.Slug) < 3 || len(in.Slug) > 60 || !slugRe.MatchString(in.Slug):
		errs["slug"] = "Usa de 3 a 60 letras minúsculas, números y guiones."
	case slices.Contains(reserved, in.Slug):
		errs["slug"] = "Esa dirección está reservada; elige otra."
	}
	if !slices.Contains(Verticals, in.Vertical) {
		errs["vertical"] = "Elige un giro válido."
	}
	if in.Timezone != "" {
		if _, err := time.LoadLocation(in.Timezone); err != nil {
			errs["timezone"] = "Zona horaria inválida."
		}
	}
	return errs
}

//go:embed templates/*.json
var templateFS embed.FS

type Template struct {
	Kind     string   `json:"kind"`     // engine: service | product | rental
	Features []string `json:"features"` // on by default; see Feature in packages/shared/src/verticals/types.ts
	// Offered by the giro but off until the owner turns them on (e.g. home service for a barbershop).
	OptionalFeatures []string       `json:"optional_features"`
	Settings         map[string]any `json:"settings"`
	Hours            []struct {
		Weekdays []int  `json:"weekdays"`
		OpensAt  string `json:"opens_at"`
		ClosesAt string `json:"closes_at"`
	} `json:"hours"`
	Items []TemplateItem `json:"items"`
	Steps []TemplateStep `json:"steps"`
	// OwnerAttends: the owner is also the person who attends (single-person business);
	// they get a staff record with every service and the business hours.
	OwnerAttends bool `json:"owner_attends"`
}

// OwnerStaffName is the default name of the owner's staff record; they rename it later.
const OwnerStaffName = "Yo"

type TemplateItem struct {
	Kind            string  `json:"kind"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Price           int64   `json:"price"`
	DurationMinutes *int    `json:"duration_minutes"`
	BufferMinutes   int     `json:"buffer_minutes"`
	PrepHours       *int    `json:"prep_hours"`
	PriceUnit       *string `json:"price_unit"`
	SecurityDeposit *int64  `json:"security_deposit"`
	LateFeePerDay   *int64  `json:"late_fee_per_day"`
	Units           int     `json:"units"` // rentals: physical pieces to create (codes like ROT-01)
}

type TemplateStep struct {
	AppliesToKind string         `json:"applies_to_kind"`
	Moment        string         `json:"moment"`
	ToStatus      string         `json:"to_status"`
	StepType      string         `json:"step_type"`
	Label         string         `json:"label"`
	Required      bool           `json:"required"`
	Config        map[string]any `json:"config"`
	Active        *bool          `json:"active"` // default true; optional steps the owner turns on in Ajustes
}

func TemplateFor(vertical string) (Template, error) {
	t, ok := templates[vertical]
	if !ok {
		return t, fmt.Errorf("template %s: unknown giro", vertical)
	}
	return t, nil
}

type Business struct {
	ID       string `json:"business_id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Vertical string `json:"vertical"`
	Role     string `json:"role"`
}

// ErrNotFound: no business with that id.
var ErrNotFound = errors.New("business not found")

// Delete removes a business with everything in it: the database cascades from businesses (bookings,
// customers, catalog, staff, finances…), and purge enqueues the job that deletes its files in S3, in
// the same transaction, so either both happen or neither. The owner's account and plan stay.
func Delete(ctx context.Context, db Beginner, businessID string, purge func(context.Context, pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Bookings and sales first: they point at staff, units, items and products without a cascade,
	// and the order of the cascade from businesses isn't guaranteed.
	for _, q := range []string{
		`delete from public.bookings where business_id = $1`,
		`delete from public.sales where business_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, businessID); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `delete from public.businesses where id = $1`, businessID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := purge(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Create registers the business, makes userID its owner and applies the template, atomically.
func Create(ctx context.Context, db Beginner, userID string, in Input) (Business, error) {
	tpl, err := TemplateFor(in.Vertical)
	if err != nil {
		return Business{}, err
	}
	settings, _ := json.Marshal(tpl.Settings)

	tx, err := db.Begin(ctx)
	if err != nil {
		return Business{}, err
	}
	defer tx.Rollback(ctx)

	b := Business{Slug: in.Slug, Name: in.Name, Vertical: in.Vertical, Role: "owner"}
	err = tx.QueryRow(ctx, `
		insert into public.businesses (slug, name, vertical, timezone, settings)
		values ($1, $2, $3, $4, $5) returning id::text`,
		in.Slug, in.Name, in.Vertical, in.Timezone, settings).Scan(&b.ID)
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Business{}, ErrSlugTaken
	}
	if err != nil {
		return Business{}, err
	}

	if _, err := tx.Exec(ctx,
		`insert into public.business_members (business_id, user_id, role) values ($1, $2, 'owner')`,
		b.ID, userID); err != nil {
		return Business{}, err
	}
	// A new owner starts their Pro trial; one who already has an account keeps their plan.
	if _, err := tx.Exec(ctx, `insert into public.account_plans (user_id, plan, ends_at)
		values ($1, 'trial', now() + make_interval(days => $2)) on conflict do nothing`, userID, plans.TrialDays); err != nil {
		return Business{}, err
	}
	if _, err := tx.Exec(ctx, `update public.businesses set plan = (select plan from public.account_plans where user_id = $2) where id = $1`,
		b.ID, userID); err != nil {
		return Business{}, err
	}

	var ownerStaff *string
	if tpl.OwnerAttends {
		var id string
		if err := tx.QueryRow(ctx, `insert into public.staff (business_id, name) values ($1, $2) returning id::text`, b.ID, OwnerStaffName).Scan(&id); err != nil {
			return Business{}, err
		}
		if _, err := tx.Exec(ctx, `update public.business_members set staff_id = $3 where business_id = $1 and user_id = $2`, b.ID, userID, id); err != nil {
			return Business{}, err
		}
		ownerStaff = &id
	}

	// General hours, and the same shifts for the owner when they attend.
	hourOwners := []*string{nil}
	if ownerStaff != nil {
		hourOwners = append(hourOwners, ownerStaff)
	}
	for _, h := range tpl.Hours {
		for _, d := range h.Weekdays {
			for _, staffID := range hourOwners {
				if _, err := tx.Exec(ctx,
					`insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at) values ($1, $2, $3, $4, $5)`,
					b.ID, staffID, d, h.OpensAt, h.ClosesAt); err != nil {
					return Business{}, err
				}
			}
		}
	}

	for i, it := range tpl.Items {
		var itemID string
		if err := tx.QueryRow(ctx, `
			insert into public.catalog_items
			  (business_id, kind, name, description, price, sort, duration_minutes, buffer_minutes,
			   prep_hours, price_unit, security_deposit, late_fee_per_day)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) returning id::text`,
			b.ID, it.Kind, it.Name, it.Description, it.Price, i+1, it.DurationMinutes, it.BufferMinutes,
			it.PrepHours, it.PriceUnit, it.SecurityDeposit, it.LateFeePerDay).Scan(&itemID); err != nil {
			return Business{}, err
		}
		for n := 1; n <= it.Units; n++ {
			if _, err := tx.Exec(ctx, `insert into public.item_units (business_id, item_id, code) values ($1, $2, $3)`,
				b.ID, itemID, catalog.UnitCode(it.Name, n)); err != nil {
				return Business{}, err
			}
		}
	}

	if ownerStaff != nil {
		if _, err := tx.Exec(ctx, `insert into public.staff_services (business_id, staff_id, item_id)
			select business_id, $2, id from public.catalog_items where business_id = $1 and kind = 'service'`, b.ID, *ownerStaff); err != nil {
			return Business{}, err
		}
	}

	for i, s := range tpl.Steps {
		cfg, _ := json.Marshal(s.Config)
		if _, err := tx.Exec(ctx, `
			insert into public.workflow_steps
			  (business_id, applies_to_kind, actor, moment, to_status, step_type, label, config, required, sort, active)
			values ($1, $2, 'business', $3, nullif($4, ''), $5, $6, $7, $8, $9, $10)`,
			b.ID, s.AppliesToKind, s.Moment, s.ToStatus, s.StepType, s.Label, cfg, s.Required, i+1, s.Active == nil || *s.Active); err != nil {
			return Business{}, err
		}
	}

	return b, tx.Commit(ctx)
}
