package features

import (
	"slices"
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
)

func TestAvailabilityAndOwnerChoice(t *testing.T) {
	// Rental giro brings guarantor and inventory; nails brings staff.
	if got := Active("tool_rental", nil, nil); !slices.Contains(got, "guarantor") || !slices.Contains(got, "inventory") || slices.Contains(got, "staff") {
		t.Fatalf("rental defaults: %v", got)
	}
	cases := []struct {
		name            string
		vertical        string
		grants, enabled map[string]bool
		key             string
		active          bool
	}{
		{"giro feature, owner leaves it on", "tool_rental", nil, nil, "guarantor", true},
		{"owner turns it off", "tool_rental", nil, map[string]bool{"guarantor": false}, "guarantor", false},
		{"admin removes it", "tool_rental", map[string]bool{"guarantor": false}, nil, "guarantor", false},
		{"admin gives one the giro lacks", "nails", map[string]bool{"customerDocuments": true}, nil, "customerDocuments", true},
		{"given, but the owner keeps it off", "nails", map[string]bool{"customerDocuments": true}, map[string]bool{"customerDocuments": false}, "customerDocuments", false},
		{"owner can't turn on what isn't available", "nails", nil, map[string]bool{"guarantor": true}, "guarantor", false},
		{"engine doesn't support it", "nails", map[string]bool{"inventory": true}, nil, "inventory", false},
		{"unknown feature", "tool_rental", map[string]bool{"magic": true}, nil, "magic", false},
		{"offered by the giro, off until the owner turns it on", "barbershop", nil, nil, "homeService", false},
		{"owner turns on an offered one", "barbershop", nil, map[string]bool{"homeService": true}, "homeService", true},
		{"barbershop takes walk-ins without starting the service", "barbershop", nil, nil, "walkIn", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := slices.Contains(Active(tc.vertical, tc.grants, tc.enabled), tc.key); got != tc.active {
				t.Fatalf("%s active = %v, want %v", tc.key, got, tc.active)
			}
		})
	}
}

func TestStatesOnlyListSupportedFeatures(t *testing.T) {
	for _, s := range States("nails", map[string]bool{"guarantor": true}, nil) {
		if s.Key == "inventory" {
			t.Fatal("inventory is rental-only")
		}
		if s.Key == "guarantor" && (s.FromGiro || s.Grant == nil || !*s.Grant || !s.Available) {
			t.Fatalf("granted guarantor: %+v", s)
		}
	}
	if m := Parse([]byte(`not json`)); len(m) != 0 {
		t.Fatal("bad input should be empty")
	}
}

// Every giro only brings features that exist and that its engine supports.
func TestGiroFeaturesAreSupported(t *testing.T) {
	for _, v := range businesses.Verticals {
		tpl, _ := businesses.TemplateFor(v)
		for _, f := range append(tpl.Features, tpl.OptionalFeatures...) {
			if !Supports(v, f) {
				t.Errorf("%s brings %q, unknown or not for its engine", v, f)
			}
		}
	}
}

func TestThePlanTurnsFeaturesOff(t *testing.T) {
	// Lite has no team, product sales or extra documents; the platform admin's explicit gift still wins.
	g := ParseForPlan([]byte(`{"productSales": true, "guarantor": false}`), "lite")
	if v, ok := g["staff"]; !ok || v {
		t.Fatalf("staff should be removed by the plan: %v", g)
	}
	if !g["productSales"] || g["guarantor"] || g["bookingDocuments"] {
		t.Fatalf("grants: %v", g)
	}
	if active := Active("barbershop", ParseForPlan(nil, "lite"), nil); slices.Contains(active, "staff") || slices.Contains(active, "productSales") {
		t.Fatalf("lite barbershop: %v", active)
	}
	if active := Active("barbershop", ParseForPlan(nil, "trial"), nil); !slices.Contains(active, "staff") {
		t.Fatalf("the trial is Pro: %v", active)
	}
}
