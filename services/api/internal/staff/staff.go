// Package staff manages the people who attend services and the opening hours
// (of the business and of each person). Times are wall-clock in the business time zone.
package staff

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("staff not found")

// Hours is one shift: weekday 0 = domingo, times "HH:MM".
type Hours struct {
	Weekday  int    `json:"weekday"`
	OpensAt  string `json:"opens_at"`
	ClosesAt string `json:"closes_at"`
}

var hhmm = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

const maxShiftsPerDay = 4

// ValidateHours returns "" when valid, else the reason (es-MX).
func ValidateHours(hs []Hours) string {
	byDay := map[int][]Hours{}
	for _, h := range hs {
		if h.Weekday < 0 || h.Weekday > 6 {
			return "Día de la semana inválido."
		}
		if !hhmm.MatchString(h.OpensAt) || !hhmm.MatchString(h.ClosesAt) {
			return "Usa el formato de 24 horas HH:MM, por ejemplo 09:30."
		}
		if h.ClosesAt <= h.OpensAt { // "HH:MM" compares correctly as text
			return "La hora de cierre debe ser después de la de apertura."
		}
		byDay[h.Weekday] = append(byDay[h.Weekday], h)
	}
	for _, day := range byDay {
		if len(day) > maxShiftsPerDay {
			return fmt.Sprintf("Máximo %d turnos por día.", maxShiftsPerDay)
		}
		sort.Slice(day, func(i, j int) bool { return day[i].OpensAt < day[j].OpensAt })
		for i := 1; i < len(day); i++ {
			if day[i].OpensAt < day[i-1].ClosesAt {
				return "Hay turnos que se enciman el mismo día."
			}
		}
	}
	return ""
}

// Member is an employee who attends services.
type Member struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Active     bool     `json:"active"`
	Color      string   `json:"color"`
	IsOwner    bool     `json:"is_owner"` // linked to the owner's account (single-person business)
	ServiceIDs []string `json:"service_ids"`
	Hours      []Hours  `json:"hours"`
}

type Patch struct {
	Name   *string `json:"name"`
	Active *bool   `json:"active"`
	Color  *string `json:"color"`
}

func (m Member) Apply(p Patch) Member {
	if p.Name != nil {
		m.Name = *p.Name
	}
	if p.Active != nil {
		m.Active = *p.Active
	}
	if p.Color != nil {
		m.Color = *p.Color
	}
	m.Name = strings.TrimSpace(m.Name)
	m.Color = strings.TrimSpace(m.Color)
	return m
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (m Member) Validate() map[string]string {
	errs := map[string]string{}
	if n := utf8.RuneCountInString(strings.TrimSpace(m.Name)); n == 0 || n > 60 {
		errs["name"] = "Escribe el nombre (máximo 60 caracteres)."
	}
	if m.Color != "" && !hexColor.MatchString(m.Color) {
		errs["color"] = "Color inválido; usa el formato #RRGGBB."
	}
	return errs
}

// ---------------------------------------------------------------- repo

type Repo struct{ DB store.DB }

// List returns the business's staff with their services and hours.
func (r Repo) List(ctx context.Context, businessID string) ([]Member, error) {
	rows, err := r.DB.Query(ctx, `
		select s.id::text, s.name, s.active, coalesce(s.color, ''),
		       exists (select 1 from public.business_members m where m.staff_id = s.id and m.role = 'owner'),
		       coalesce((select array_agg(ss.item_id::text order by ss.item_id) from public.staff_services ss where ss.staff_id = s.id), '{}'),
		       coalesce((select jsonb_agg(jsonb_build_object('weekday', h.weekday, 'opens_at', to_char(h.opens_at, 'HH24:MI'),
		                 'closes_at', to_char(h.closes_at, 'HH24:MI')) order by h.weekday, h.opens_at)
		                 from public.business_hours h where h.staff_id = s.id), '[]')
		from public.staff s
		where s.business_id = $1
		order by s.active desc, s.created_at`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Active, &m.Color, &m.IsOwner, &m.ServiceIDs, &m.Hours); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r Repo) Get(ctx context.Context, businessID, staffID string) (Member, error) {
	all, err := r.List(ctx, businessID)
	if err != nil {
		return Member{}, err
	}
	for _, m := range all {
		if m.ID == staffID {
			return m, nil
		}
	}
	return Member{}, ErrNotFound
}

func (r Repo) Create(ctx context.Context, businessID string, m Member) (Member, error) {
	err := r.DB.QueryRow(ctx,
		`insert into public.staff (business_id, name, color) values ($1, $2, nullif($3, '')) returning id::text, active`,
		businessID, m.Name, m.Color).Scan(&m.ID, &m.Active)
	m.ServiceIDs, m.Hours = []string{}, []Hours{}
	return m, err
}

func (r Repo) Update(ctx context.Context, businessID string, m Member) error {
	tag, err := r.DB.Exec(ctx, `update public.staff set name = $3, active = $4, color = nullif($5, '') where business_id = $1 and id = $2`,
		businessID, m.ID, m.Name, m.Active, m.Color)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// ErrNotAService: some item id is not a service of this business.
var ErrNotAService = errors.New("item is not a service of this business")

// SetServices replaces which services the person attends.
func (r Repo) SetServices(ctx context.Context, businessID, staffID string, itemIDs []string) error {
	return inTx(ctx, r.DB, func(tx pgx.Tx) error {
		if err := ownStaff(ctx, tx, businessID, staffID); err != nil {
			return err
		}
		var valid int
		if err := tx.QueryRow(ctx, `select count(*) from public.catalog_items where business_id = $1 and kind = 'service' and id::text = any($2)`,
			businessID, itemIDs).Scan(&valid); err != nil {
			return err
		}
		if valid != len(unique(itemIDs)) {
			return ErrNotAService
		}
		if _, err := tx.Exec(ctx, `delete from public.staff_services where staff_id = $1`, staffID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			insert into public.staff_services (business_id, staff_id, item_id)
			select $1, $2, i::uuid from unnest($3::text[]) i on conflict do nothing`, businessID, staffID, itemIDs)
		return err
	})
}

// SetHours replaces the hours of a person (staffID != "") or the general hours of the business ("").
func (r Repo) SetHours(ctx context.Context, businessID, staffID string, hs []Hours) error {
	return inTx(ctx, r.DB, func(tx pgx.Tx) error {
		if staffID != "" {
			if err := ownStaff(ctx, tx, businessID, staffID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `delete from public.business_hours where business_id = $1 and staff_id is not distinct from nullif($2, '')::uuid`,
			businessID, staffID); err != nil {
			return err
		}
		for _, h := range hs {
			if _, err := tx.Exec(ctx, `insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at)
				values ($1, nullif($2, '')::uuid, $3, $4, $5)`, businessID, staffID, h.Weekday, h.OpensAt, h.ClosesAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// BusinessHours returns the general hours (not tied to a person).
func (r Repo) BusinessHours(ctx context.Context, businessID string) ([]Hours, error) {
	rows, err := r.DB.Query(ctx, `select weekday, to_char(opens_at, 'HH24:MI'), to_char(closes_at, 'HH24:MI')
		from public.business_hours where business_id = $1 and staff_id is null order by weekday, opens_at`, businessID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Hours])
}

func ownStaff(ctx context.Context, tx pgx.Tx, businessID, staffID string) error {
	var ok bool
	if err := tx.QueryRow(ctx, `select exists (select 1 from public.staff where business_id = $1 and id = $2)`, businessID, staffID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func inTx(ctx context.Context, db store.DB, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func unique(ids []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
