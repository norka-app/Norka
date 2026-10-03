package main

import "testing"

func TestAppVersionMatchesWailsConfig(t *testing.T) {
	if got := appVersion(); got != "1.5.0" {
		t.Fatalf("appVersion() = %q, want 1.5.0 from wails.json", got)
	}
}
