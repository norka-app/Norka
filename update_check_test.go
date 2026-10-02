package main

import "testing"

func TestAppVersionMatchesWailsConfig(t *testing.T) {
	if got := appVersion(); got != "1.2.0" {
		t.Fatalf("appVersion() = %q, want 1.2.0 from wails.json", got)
	}
}
