package automation

import (
	"errors"
	"testing"

	"github.com/norka-app/Norka/internal/model"
)

func TestMatchExactThenIDThenPrefix(t *testing.T) {
	tunnels := []model.Tunnel{
		{ID: 1, Name: "db"},
		{ID: 2, Name: "db-prod"},
		{ID: 10, Name: "API"},
	}

	got, err := Match(tunnels, "DB")
	if err != nil || got.ID != 1 {
		t.Fatalf("exact name: %+v %v", got, err)
	}
	got, err = Match(tunnels, "10")
	if err != nil || got.Name != "API" {
		t.Fatalf("id should win over a missing exact name: %+v %v", got, err)
	}
	got, err = Match(tunnels, "db-p")
	if err != nil || got.ID != 2 {
		t.Fatalf("unique prefix: %+v %v", got, err)
	}
	got, err = Match(tunnels, "2")
	if err != nil || got.Name != "db-prod" {
		t.Fatalf("id: %+v %v", got, err)
	}

	_, err = Match(tunnels, "d")
	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) || len(ambiguous.Names()) != 2 {
		t.Fatalf("prefix ambiguity: %v", err)
	}
	_, err = Match(tunnels, "missing")
	if err == nil || errors.As(err, &ambiguous) {
		t.Fatalf("missing: %v", err)
	}
	if _, err = Match(tunnels, "01"); err == nil {
		t.Fatal("leading zero must not match id 1")
	}
	if _, err = Match(tunnels, ""); err == nil {
		t.Fatal("empty query")
	}
}

func TestMatchDuplicateExactNameDoesNotFallThrough(t *testing.T) {
	tunnels := []model.Tunnel{
		{ID: 1, Name: "db"},
		{ID: 2, Name: "DB"},
	}
	_, err := Match(tunnels, "db")
	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("duplicate exact name: %v", err)
	}
}
