package conf

import "testing"

func TestAutomationTrustDropsUnknownTunnels(t *testing.T) {
	raw := []byte(`
version = 1
automation_trusted = [1, 2, 2, 9]

[[tunnels]]
id = 1
name = "db"
mode = "local"
local_host = "127.0.0.1"
local_port = 5432
remote_host = "10.0.0.8"
remote_port = 5432
status = "stopped"
`)
	cfg, err := ParseConfigTOML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutomationAllows(1) || cfg.AutomationAllows(2) || cfg.AutomationAllows(9) {
		t.Fatalf("trust: %+v", cfg.AutomationTrusted)
	}
	cfg.TrustAutomationTunnel(1)
	if len(cfg.AutomationTrusted) != 1 {
		t.Fatalf("duplicate trust: %+v", cfg.AutomationTrusted)
	}
	cloned := cfg.Clone()
	cfg.TrustAutomationTunnel(1)
	cloned.AutomationTrusted = append(cloned.AutomationTrusted, 99)
	if cfg.AutomationAllows(99) {
		t.Fatal("clone aliased the trust list")
	}
}
