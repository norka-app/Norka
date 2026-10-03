package ipc

import "testing"

func TestGateVersion(t *testing.T) {
	if got := GateVersion(0); got.Reject {
		t.Fatal("legacy client must be accepted")
	}
	if got := GateVersion(ProtocolVersion); got.Reject {
		t.Fatal("current client must be accepted")
	}
	newer := GateVersion(ProtocolVersion + 1)
	if !newer.Reject || newer.Response.Code != CodeUpdateApp || newer.Response.ExitCode != ExitVersion {
		t.Fatalf("newer client: %+v", newer)
	}
	if newer.Response.Message != MessageUpdateApp {
		t.Fatalf("message: %q", newer.Response.Message)
	}

	// A future app can raise the minimum. v=0 stays accepted; a numbered
	// dialect below the minimum asks for a newer norka-cli.
	older := gateVersion(1, 2, 2)
	if !older.Reject || older.Response.Code != CodeUpdateCLI || older.Response.Message != MessageUpdateCLI {
		t.Fatalf("older client: %+v", older)
	}
	if got := gateVersion(0, 2, 2); got.Reject {
		t.Fatal("legacy client must stay accepted after the minimum moves")
	}
	if got := gateVersion(2, 2, 2); got.Reject {
		t.Fatal("matching future dialect must be accepted")
	}
	if got := gateVersion(-1, 1, 1); !got.Reject || got.Response.Code != CodeBadRequest {
		t.Fatalf("negative: %+v", got)
	}
}
