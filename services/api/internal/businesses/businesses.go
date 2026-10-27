// Package businesses registers a business and applies its vertical's starting template.
package businesses

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var Verticals = []string{"nails", "flowers", "tool_rental"}

// KindFor is the catalog item kind each vertical sells.
func KindFor(vertical string) string {
	switch vertical {
	case "nails":
		return "service"
	case "flowers":
		return "product"
	default:
		return "rental"
	}
}

// Slugs that would collide with app routes, since the public catalog lives at /<slug>.
var reserved = []string{"admin", "login", "signup", "onboarding", "api", "app", "assets", "static", "www", "help", "soporte", "_expo"}

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
	Settings map[string]any `json:"settings"`
	Hours    []struct {
		Weekdays []int  `json:"weekdays"`
		OpensAt  string `json:"opens_at"`
		ClosesAt string `json:"closes_at"`
	} `json:"hours"`
	Items []TemplateItem `json:"items"`
	Steps []TemplateStep `json:"steps"`
}

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
}

type TemplateStep struct {
	AppliesToKind string         `json:"applies_to_kind"`
	Moment        string         `json:"moment"`
	ToStatus      string         `json:"to_status"`
	StepType      string         `json:"step_type"`
	Label         string         `json:"label"`
	Required      bool           `json:"required"`
	Config        map[string]any `json:"config"`
}

func TemplateFor(vertical string) (Template, error) {
	var t Template
	raw, err := templateFS.ReadFile("templates/" + vertical + ".json")
	if err != nil {
		return t, fmt.Errorf("template %s: %w", vertical, err)
	}
	return t, json.Unmarshal(raw, &t)
}

type Business struct {
	ID       string `json:"business_id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Vertical string `json:"vertical"`
	Role     string `json:"role"`
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

	for _, h := range tpl.Hours {
		for _, d := range h.Weekdays {
			if _, err := tx.Exec(ctx,
				`insert into public.business_hours (business_id, weekday, opens_at, closes_at) values ($1, $2, $3, $4)`,
				b.ID, d, h.OpensAt, h.ClosesAt); err != nil {
				return Business{}, err
			}
		}
	}

	for i, it := range tpl.Items {
		if _, err := tx.Exec(ctx, `
			insert into public.catalog_items
			  (business_id, kind, name, description, price, sort, duration_minutes, buffer_minutes,
			   prep_hours, price_unit, security_deposit, late_fee_per_day)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			b.ID, it.Kind, it.Name, it.Description, it.Price, i+1, it.DurationMinutes, it.BufferMinutes,
			it.PrepHours, it.PriceUnit, it.SecurityDeposit, it.LateFeePerDay); err != nil {
			return Business{}, err
		}
	}

	for i, s := range tpl.Steps {
		cfg, _ := json.Marshal(s.Config)
		if _, err := tx.Exec(ctx, `
			insert into public.workflow_steps
			  (business_id, applies_to_kind, actor, moment, to_status, step_type, label, config, required, sort)
			values ($1, $2, 'business', $3, nullif($4, ''), $5, $6, $7, $8, $9)`,
			b.ID, s.AppliesToKind, s.Moment, s.ToStatus, s.StepType, s.Label, cfg, s.Required, i+1); err != nil {
			return Business{}, err
		}
	}

	return b, tx.Commit(ctx)
}
