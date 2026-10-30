package catalog

import (
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
)

func TestEveryEngineHasItemFields(t *testing.T) {
	if missing, extra := engines.Covers(rulesByKind); missing != nil || extra != nil {
		t.Fatalf("rulesByKind: missing %v, extra %v", missing, extra)
	}
	for kind, r := range rulesByKind {
		for _, g := range r.fields {
			if g.validate == nil || g.clear == nil {
				t.Errorf("%s: a field group needs validate and clear", kind)
			}
		}
	}
	if !HasUnits(engines.Rental) || HasUnits(engines.Service) || HasUnits("unknown") {
		t.Fatal("only rentals have units")
	}
}

func TestUnitsMatchEnginesJSON(t *testing.T) {
	for kind, m := range engines.ModulesOf {
		if HasUnits(kind) != (m.Resource == "units") {
			t.Errorf("%s: units %v, engines.json resource %q", kind, HasUnits(kind), m.Resource)
		}
	}
}
