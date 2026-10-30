package engines_test

import (
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/businesses"
	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
)

// Every giro books with a real engine.
func TestEveryGiroUsesAKnownEngine(t *testing.T) {
	for _, v := range businesses.Verticals {
		if k := businesses.KindFor(v); !engines.Valid(k) {
			t.Errorf("giro %s: engine %q is not in engines.Kinds", v, k)
		}
	}
}
