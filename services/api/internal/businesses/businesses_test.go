package businesses

import (
	"slices"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := Input{Name: "Uñas Bellas", Slug: "unas-bellas", Vertical: "nails"}

	cases := []struct {
		name      string
		mut       func(*Input)
		wantField string // "" = valid
	}{
		{"válido", func(*Input) {}, ""},
		{"nombre vacío", func(i *Input) { i.Name = "   " }, "name"},
		{"nombre muy largo", func(i *Input) { i.Name = strings.Repeat("a", 81) }, "name"},
		{"slug con mayúsculas", func(i *Input) { i.Slug = "Unas" }, "slug"},
		{"slug con espacios", func(i *Input) { i.Slug = "unas bellas" }, "slug"},
		{"slug muy corto", func(i *Input) { i.Slug = "ab" }, "slug"},
		{"slug con guion al final", func(i *Input) { i.Slug = "unas-" }, "slug"},
		{"slug reservado", func(i *Input) { i.Slug = "admin" }, "slug"},
		{"giro desconocido", func(i *Input) { i.Vertical = "limos" }, "vertical"},
		{"zona horaria inválida", func(i *Input) { i.Timezone = "Marte/Base" }, "timezone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ok
			tc.mut(&in)
			errs := in.Validate()
			if tc.wantField == "" {
				if len(errs) != 0 {
					t.Fatalf("want valid, got %v", errs)
				}
				return
			}
			if _, has := errs[tc.wantField]; !has {
				t.Fatalf("want error on %q, got %v", tc.wantField, errs)
			}
		})
	}
}

func TestNormalizeDefaultsAndTrims(t *testing.T) {
	in := Input{Name: "  Florería  ", Slug: " flores-centro ", Vertical: "flowers"}.Normalize()
	if in.Name != "Florería" || in.Slug != "flores-centro" || in.Timezone != "America/Chihuahua" {
		t.Fatalf("got %+v", in)
	}
}

func TestTemplatesAreCompleteForEachVertical(t *testing.T) {
	for _, v := range Verticals {
		t.Run(v, func(t *testing.T) {
			tpl, err := TemplateFor(v)
			if err != nil {
				t.Fatal(err)
			}
			if len(tpl.Hours) == 0 || len(tpl.Items) == 0 {
				t.Fatalf("template %s needs hours and items", v)
			}
			if _, ok := tpl.Settings["deposit_percent"]; !ok {
				t.Errorf("template %s settings without deposit_percent", v)
			}
			for _, it := range tpl.Items {
				if it.Kind != KindFor(v) {
					t.Errorf("%s: item %q is %s, want %s", v, it.Name, it.Kind, KindFor(v))
				}
				if it.Price <= 0 {
					t.Errorf("%s: item %q price %d", v, it.Name, it.Price)
				}
			}
		})
	}
}

func TestRentalTemplateRequiresIDBeforeCheckout(t *testing.T) {
	tpl, _ := TemplateFor("tool_rental")
	for _, s := range tpl.Steps {
		if s.ToStatus == "checked_out" && s.Label == "Foto del INE" && s.Required {
			return
		}
	}
	t.Fatal("rental template must require the INE photo before checked_out")
}

func TestRentalTemplateComesWithUnits(t *testing.T) {
	tpl, _ := TemplateFor("tool_rental")
	for _, it := range tpl.Items {
		if it.Units < 1 {
			t.Errorf("%s has no units: nothing could be rented", it.Name)
		}
	}
}

func TestEveryGiroHasAKnownEngineAndFeatures(t *testing.T) {
	known := []string{"staff", "inventory", "guarantor", "customerAddress", "customerDocuments", "trustedCustomers"}
	if len(Verticals) < 3 {
		t.Fatalf("giros: %v", Verticals)
	}
	for _, v := range Verticals {
		if !slices.Contains([]string{"service", "product", "rental"}, KindFor(v)) {
			t.Errorf("%s: engine %q", v, KindFor(v))
		}
		for _, f := range templates[v].Features {
			if !slices.Contains(known, f) {
				t.Errorf("%s: unknown feature %q", v, f)
			}
		}
	}
	if !HasFeature("tool_rental", "guarantor") || HasFeature("nails", "guarantor") || KindFor("lavado") != "" {
		t.Fatal("feature lookup")
	}
}
