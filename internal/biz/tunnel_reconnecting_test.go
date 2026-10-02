package biz

import "testing"

func TestListReportsStaleReconnectingAsError(t *testing.T) {
	tunnelBiz := createAutoStartTunnels(t, 1)
	cfg, err := tunnelBiz.storage.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	id := cfg.Tunnels[0].ID

	if _, err := tunnelBiz.updateStatus(id, statusReconnecting, "ssh connection closed"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	// no runtime alive -> a persisted "reconnecting" is stale and shown as "error"
	items, err := tunnelBiz.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items[0].Status != "error" || items[0].LastError != "ssh connection closed" {
		t.Fatalf("stale reconnecting = %q (%q), want error with last error kept", items[0].Status, items[0].LastError)
	}

	// runtime alive -> "reconnecting" is reported as is
	tunnelBiz.mu.Lock()
	tunnelBiz.runs[id] = nil
	tunnelBiz.mu.Unlock()
	items, err = tunnelBiz.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items[0].Status != statusReconnecting {
		t.Fatalf("live reconnecting = %q, want %q", items[0].Status, statusReconnecting)
	}
}
