// Package engines is the single list of booking engines (catalog item kind): how something is
// booked. Each layer (catalog, availability, bookings) keeps its rules per engine in a table keyed
// by these kinds, and a test in each one checks the table covers exactly this list. So a new
// engine starts here, fails those tests until every layer implements it, and an unknown kind is an
// error instead of falling into another engine's rules. The database checks the same list
// (catalog_items.kind, bookings.kind), and packages/shared mirrors it.
package engines

import (
	"fmt"
	"maps"
	"slices"
)

const (
	Service = "service" // a person's time slot (duration + buffer)
	Product = "product" // a daily capacity with lead time; pick-up or delivery
	Rental  = "rental"  // physical units over a date range
)

// Kinds lists every engine.
var Kinds = []string{Service, Product, Rental}

func Valid(kind string) bool { return slices.Contains(Kinds, kind) }

// ErrUnknown is returned when a kind has no rules in a layer.
type ErrUnknown struct{ Kind string }

func (e ErrUnknown) Error() string { return fmt.Sprintf("engines: unknown kind %q", e.Kind) }

// Get returns the layer's rules for a kind, or ErrUnknown.
func Get[T any](table map[string]T, kind string) (T, error) {
	v, ok := table[kind]
	if !ok {
		return v, ErrUnknown{kind}
	}
	return v, nil
}

// Covers reports the kinds a layer's table is missing and the ones it has that aren't engines.
func Covers[T any](table map[string]T) (missing, extra []string) {
	for _, k := range Kinds {
		if _, ok := table[k]; !ok {
			missing = append(missing, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(table)) {
		if !Valid(k) {
			extra = append(extra, k)
		}
	}
	return missing, extra
}
