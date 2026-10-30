package bookings

import (
	"testing"

	"github.com/omaradriano/local_bussiness/services/api/internal/engines"
)

func TestEveryEngineHasAStateMachine(t *testing.T) {
	if missing, extra := engines.Covers(transitions); missing != nil || extra != nil {
		t.Fatalf("transitions: missing %v, extra %v", missing, extra)
	}
}
