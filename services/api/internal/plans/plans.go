// Package plans: what each subscription includes (plans.json, also read by the app) and the
// owner's account plan. A plan belongs to the owner's account and covers all their businesses.
// There's no free plan: a new owner gets a Pro trial; when it (or a paid plan) ends, the account
// is read-only until the platform admin assigns a plan. No end date = pilot or courtesy.
package plans

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"
)

const (
	Trial    = "trial"
	Lite     = "lite"
	Pro      = "pro"
	Business = "business"

	StatusTrial   = "trial"
	StatusActive  = "active"
	StatusExpired = "expired"
)

// Capabilities beyond the giro's features (parts of Finanzas and reports).
var Capabilities = []string{"expenses", "recurring_expenses", "installments", "payroll", "any_period", "compare_businesses"}

// Limits; nil = unlimited.
type Limits struct {
	Businesses    *int `json:"businesses"`
	Members       *int `json:"members"` // people who sign in
	Staff         *int `json:"staff"`   // people who attend (agenda)
	CatalogItems  *int `json:"catalog_items"`
	RentalUnits   *int `json:"rental_units"`
	PhotosPerItem *int `json:"photos_per_item"`
}

type Def struct {
	Limits           Limits   `json:"limits"`
	ExcludedFeatures []string `json:"excluded_features"`
	Capabilities     []string `json:"capabilities"`
}

//go:embed plans.json
var raw []byte

var catalog = func() (c struct {
	Trial struct {
		Days int    `json:"days"`
		As   string `json:"as"`
	} `json:"trial"`
	Plans map[string]Def `json:"plans"`
}) {
	if err := json.Unmarshal(raw, &c); err != nil {
		panic(err)
	}
	if _, ok := c.Plans[c.Trial.As]; !ok || c.Trial.Days < 1 {
		panic("plans.json: trial must name a plan and last at least a day")
	}
	return c
}()

var (
	// Keys of the paid plans, sorted.
	Keys      = slices.Sorted(maps.Keys(catalog.Plans))
	TrialDays = catalog.Trial.Days
)

func Valid(plan string) bool { return plan == Trial || slices.Contains(Keys, plan) }

// Get: what a plan includes (the trial includes what its plan does).
func Get(plan string) Def {
	if plan == Trial {
		plan = catalog.Trial.As
	}
	return catalog.Plans[plan]
}

// Account: the owner's plan and when it ends (nil = no end).
type Account struct {
	Plan   string     `json:"plan"`
	EndsAt *time.Time `json:"ends_at"`
}

func NewTrial(now time.Time) Account {
	end := now.AddDate(0, 0, TrialDays)
	return Account{Plan: Trial, EndsAt: &end}
}

func (a Account) Status(now time.Time) string {
	switch {
	case a.EndsAt != nil && now.After(*a.EndsAt):
		return StatusExpired
	case a.Plan == Trial:
		return StatusTrial
	default:
		return StatusActive
	}
}

// View: the account as the app shows it.
type View struct {
	Plan     string     `json:"plan"`
	Status   string     `json:"status"` // trial | active | expired
	EndsAt   *time.Time `json:"ends_at"`
	DaysLeft int        `json:"days_left"` // -1 with no end
}

func (a Account) View(now time.Time) View {
	return View{Plan: a.Plan, Status: a.Status(now), EndsAt: a.EndsAt, DaysLeft: a.DaysLeft(now)}
}

// Writable: an expired account can still read (and export) everything, but not change anything.
func (a Account) Writable(now time.Time) bool { return a.Status(now) != StatusExpired }

// DaysLeft: whole days started until it ends; 0 once ended, -1 with no end.
func (a Account) DaysLeft(now time.Time) int {
	if a.EndsAt == nil {
		return -1
	}
	if !now.Before(*a.EndsAt) {
		return 0
	}
	h := a.EndsAt.Sub(now).Hours()
	d := int(h / 24)
	if float64(d*24) < h {
		d++
	}
	return d
}

// Limit: the cap on one kind of thing (nil = unlimited, or not a limit at all).
func (d Def) Limit(what string) *int {
	switch what {
	case "businesses":
		return d.Limits.Businesses
	case "members":
		return d.Limits.Members
	case "staff":
		return d.Limits.Staff
	case "catalog_items":
		return d.Limits.CatalogItems
	case "rental_units":
		return d.Limits.RentalUnits
	case "photos_per_item":
		return d.Limits.PhotosPerItem
	}
	return nil
}

func (d Def) Can(capability string) bool { return slices.Contains(d.Capabilities, capability) }

// HasRoom: one more fits when used is below the limit.
func HasRoom(limit *int, used int) bool { return limit == nil || used < *limit }

// limitNouns: singular and plural.
var limitNouns = map[string][2]string{
	"businesses":      {"negocio", "negocios"},
	"members":         {"persona con acceso", "personas con acceso"},
	"staff":           {"persona que atiende", "personas que atienden"},
	"catalog_items":   {"artículo en el catálogo", "artículos en el catálogo"},
	"rental_units":    {"unidad en renta", "unidades en renta"},
	"photos_per_item": {"foto por artículo", "fotos por artículo"},
}

var names = map[string]string{Lite: "Lite", Pro: "Pro", Business: "Business"}

// LimitMessage: what the owner reads when a limit stops them (es-MX).
func LimitMessage(plan, what string, limit int) string {
	who := "Tu plan " + names[plan]
	if plan == Trial {
		who = "Tu prueba (plan " + names[catalog.Trial.As] + ")"
	}
	noun := limitNouns[what][1]
	if limit == 1 {
		noun = limitNouns[what][0]
	}
	return fmt.Sprintf("%s permite hasta %d %s.", who, limit, noun)
}

// IsCalendarMonth: [from, to] is exactly one month (what plans without "any_period" may look at).
func IsCalendarMonth(from, to time.Time) bool {
	return from.Day() == 1 && to.Equal(from.AddDate(0, 1, -1))
}
