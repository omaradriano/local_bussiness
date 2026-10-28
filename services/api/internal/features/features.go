// Package features decides which parts of the app a business has.
//
//   - Its giro brings some features (template "features").
//   - The platform admin can grant or remove one for a business (businesses.feature_grants, not
//     writable by the owner): available = giro's + granted − removed, only if the engine supports it.
//   - The owner turns available ones on or off in Ajustes (settings.features; on by default).
//
// Active = available and on. The catalog of features and the engines each one works with is
// features.json, also read by the app (packages/shared).
package features

import (
	_ "embed"
	"encoding/json"
	"maps"
	"slices"

	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
)

//go:embed features.json
var catalogJSON []byte

type info struct {
	Engines []string `json:"engines"`
}

var catalog = func() map[string]info {
	var c map[string]info
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(err)
	}
	return c
}()

// Known lists every feature, sorted.
var Known = slices.Sorted(maps.Keys(catalog))

// Supports reports whether a feature exists and works with the giro's engine.
func Supports(vertical, feature string) bool {
	f, ok := catalog[feature]
	return ok && slices.Contains(f.Engines, businesses.KindFor(vertical))
}

// State of one feature for a business.
type State struct {
	Key       string `json:"key"`
	FromGiro  bool   `json:"from_giro"` // the giro brings it
	Grant     *bool  `json:"grant"`     // platform admin override: true = given, false = removed, null = as the giro
	Available bool   `json:"available"` // the owner may use it
	Enabled   bool   `json:"enabled"`   // the owner has it on (default on)
	Active    bool   `json:"active"`    // available and on
}

// States lists every feature the giro's engine supports, with its state for this business.
func States(vertical string, grants, enabled map[string]bool) []State {
	var out []State
	for _, key := range Known {
		if !Supports(vertical, key) {
			continue
		}
		s := State{Key: key, FromGiro: businesses.HasFeature(vertical, key), Enabled: true}
		s.Available = s.FromGiro
		if g, ok := grants[key]; ok {
			s.Grant = &g
			s.Available = g
		}
		if on, ok := enabled[key]; ok {
			s.Enabled = on
		}
		s.Active = s.Available && s.Enabled
		out = append(out, s)
	}
	return out
}

// Active returns the keys of the business's active features.
func Active(vertical string, grants, enabled map[string]bool) []string {
	out := []string{}
	for _, s := range States(vertical, grants, enabled) {
		if s.Active {
			out = append(out, s.Key)
		}
	}
	return out
}

// Parse reads a jsonb map of feature -> bool (grants or the owner's choices); bad input = empty.
func Parse(raw []byte) map[string]bool {
	m := map[string]bool{}
	_ = json.Unmarshal(raw, &m)
	return m
}
