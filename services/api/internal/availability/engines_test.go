package availability

import (
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
)

func TestEveryEngineHasAnAvailabilityRule(t *testing.T) {
	if missing, extra := engines.Covers(answerers); missing != nil || extra != nil {
		t.Fatalf("answerers: missing %v, extra %v", missing, extra)
	}
}
