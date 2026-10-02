package main

import "testing"

func TestOpenReportEmailIsLocalOnly(t *testing.T) {
	result := (&App{}).OpenReportEmail(OpenReportEmailPayload{
		Subject: "report",
		Body:    "details",
	})
	if result.Success {
		t.Fatal("OpenReportEmail() success = true, want local-only failure")
	}
	if result.Error == "" {
		t.Fatal("OpenReportEmail() error is empty")
	}
}
