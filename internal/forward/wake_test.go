package forward

import (
	"testing"

	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/wake"
)

func TestUnstartedForwardStaysStopped(t *testing.T) {
	forwarder := NewLocalForward(model.Tunnel{ID: 1, Name: "db"}, nil)
	if forwarder.Phase() != wake.PhaseStopped {
		t.Fatalf("phase = %q", forwarder.Phase())
	}
	if forwarder.ReconnectNow() {
		t.Fatal("a tunnel the user has not started must stay stopped")
	}
	alive, err := forwarder.Probe(wake.ProbeTimeout)
	if alive || err != nil {
		t.Fatalf("probe alive=%v err=%v", alive, err)
	}
}
