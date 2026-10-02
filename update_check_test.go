package main

import "testing"

func TestAppVersionMatchesWailsConfig(t *testing.T) {
	if got := appVersion(); got != "1.0.0" {
		t.Fatalf("appVersion() = %q, want 1.0.0 from wails.json", got)
	}
}
