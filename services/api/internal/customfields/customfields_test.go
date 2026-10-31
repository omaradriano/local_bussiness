package customfields

import (
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestFieldValidate(t *testing.T) {
	ok := Field{Kind: "rental", Target: "unit", Label: "Número de serie", Type: "text"}
	cases := []struct {
		name  string
		f     Field
		field string // "" = valid
	}{
		{"text on a unit of a rental", ok, ""},
		{"select with options", Field{Kind: "rental", Target: "item", Label: "Condición", Type: "select", Options: []string{"Nueva", "Buena"}}, ""},
		{"number on a service", Field{Kind: "service", Target: "item", Label: "Sesiones", Type: "number"}, ""},
		{"unknown engine", Field{Kind: "sale", Target: "item", Label: "X", Type: "text"}, "kind"},
		{"units only where the engine has them", Field{Kind: "service", Target: "unit", Label: "X", Type: "text"}, "target"},
		{"unknown target", Field{Kind: "rental", Target: "booking", Label: "X", Type: "text"}, "target"},
		{"empty label", Field{Kind: "rental", Target: "item", Label: "  ", Type: "text"}, "label"},
		{"long label", Field{Kind: "rental", Target: "item", Label: strings.Repeat("x", 61), Type: "text"}, "label"},
		{"unknown type", Field{Kind: "rental", Target: "item", Label: "X", Type: "photo"}, "type"},
		{"select without options", Field{Kind: "rental", Target: "item", Label: "X", Type: "select"}, "options"},
		{"select with a repeated option", Field{Kind: "rental", Target: "item", Label: "X", Type: "select", Options: []string{"A", "a "}}, "options"},
		{"select with an empty option", Field{Kind: "rental", Target: "item", Label: "X", Type: "select", Options: []string{"A", " "}}, "options"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := c.f.Normalize().Validate()
			if c.field == "" && len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if c.field != "" && errs[c.field] == "" {
				t.Fatalf("want error on %q, got %v", c.field, errs)
			}
		})
	}
}

func TestNormalizeClearsOptionsOfOtherTypes(t *testing.T) {
	f := Field{Kind: "rental", Target: "item", Label: " Marca ", Type: "text", Options: []string{"A"}}.Normalize()
	if f.Label != "Marca" || len(f.Options) != 0 {
		t.Fatalf("%+v", f)
	}
	s := Field{Type: "select", Options: []string{" Nueva ", "Usada"}}.Normalize()
	if s.Options[0] != "Nueva" {
		t.Fatalf("%+v", s)
	}
}

func TestApplyKeepsWhatIsntSent(t *testing.T) {
	f := Field{Kind: "rental", Target: "item", Label: "Marca", Type: "text", Active: true}
	g := f.Apply(Patch{Label: ptr("Fabricante"), Required: ptr(true)})
	if g.Label != "Fabricante" || !g.Required || !g.Active || g.Type != "text" || g.Target != "item" {
		t.Fatalf("%+v", g)
	}
}

func TestCleanValues(t *testing.T) {
	fields := []Field{
		{ID: "brand", Target: "item", Label: "Marca", Type: "text", Active: true, Required: true},
		{ID: "watts", Target: "item", Label: "Potencia", Type: "number", Active: true},
		{ID: "cond", Target: "item", Label: "Condición", Type: "select", Options: []string{"Nueva", "Buena"}, Active: true},
		{ID: "bought", Target: "item", Label: "Compra", Type: "date", Active: true},
		{ID: "case", Target: "item", Label: "Con estuche", Type: "boolean", Active: true},
		{ID: "serial", Target: "unit", Label: "Serie", Type: "text", Active: true},
		{ID: "old", Target: "item", Label: "Viejo", Type: "text", Active: false},
	}
	t.Run("valid values are kept, typed and trimmed", func(t *testing.T) {
		got, errs := Clean(fields, "item", map[string]any{
			"brand": " DeWalt ", "watts": 1500.0, "cond": "Buena", "bought": "2026-01-15", "case": true,
		}, nil)
		if len(errs) != 0 || got["brand"] != "DeWalt" || got["watts"] != 1500.0 || got["cond"] != "Buena" || got["bought"] != "2026-01-15" || got["case"] != true {
			t.Fatalf("%v %v", got, errs)
		}
	})
	t.Run("empty values are dropped; unknown, other-target and inactive keys too", func(t *testing.T) {
		got, errs := Clean(fields, "item", map[string]any{"brand": "Bosch", "watts": nil, "cond": "", "serial": "X1", "old": "y", "nope": 1}, nil)
		if len(errs) != 0 || len(got) != 1 || got["brand"] != "Bosch" {
			t.Fatalf("%v %v", got, errs)
		}
	})
	t.Run("an inactive field keeps its saved value", func(t *testing.T) {
		got, _ := Clean(fields, "item", map[string]any{"brand": "Bosch"}, map[string]any{"old": "guardado"})
		if got["old"] != "guardado" {
			t.Fatalf("%v", got)
		}
	})
	t.Run("required", func(t *testing.T) {
		if _, errs := Clean(fields, "item", map[string]any{"brand": "  "}, nil); errs["custom_values.brand"] == "" {
			t.Fatalf("missing required accepted: %v", errs)
		}
	})
	bad := []struct {
		id string
		v  any
	}{
		{"brand", 12.0},
		{"brand", strings.Repeat("x", 201)},
		{"watts", "mucho"},
		{"watts", 1e13},
		{"cond", "Rota"},
		{"bought", "15/01/2026"},
		{"bought", "2026-02-30"},
		{"case", "sí"},
	}
	for _, b := range bad {
		in := map[string]any{"brand": "Bosch", b.id: b.v}
		if _, errs := Clean(fields, "item", in, nil); errs["custom_values."+b.id] == "" {
			t.Errorf("%s = %v accepted", b.id, b.v)
		}
	}
	t.Run("unit fields", func(t *testing.T) {
		got, errs := Clean(fields, "unit", map[string]any{"serial": "SN-123", "brand": "x"}, nil)
		if len(errs) != 0 || len(got) != 1 || got["serial"] != "SN-123" {
			t.Fatalf("%v %v", got, errs)
		}
	})
}
