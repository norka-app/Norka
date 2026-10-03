package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboardingDonePersists(t *testing.T) {
	fresh := DefaultConfig()
	fresh.Normalize()
	if fresh.OnboardingDone {
		t.Fatal("a new config must not mark the tour done")
	}
	if strings.Contains(string(MarshalTOML(fresh)), "onboarding_done") {
		t.Fatal("missing onboarding_done must stay absent")
	}

	missing, err := ParseConfigTOML([]byte("version = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if missing.OnboardingDone {
		t.Fatal("a config without the key must stay not done")
	}

	dir := t.TempDir()
	storage, err := NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := storage.Update(func(cfg *Config) error {
		cfg.OnboardingDone = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.OnboardingDone {
		t.Fatal("update result dropped onboarding_done")
	}

	again, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !again.OnboardingDone {
		t.Fatal("onboarding_done = true did not survive save and load")
	}
	body, err := os.ReadFile(storage.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "onboarding_done = true") {
		t.Fatalf("config file missing onboarding_done = true:\n%s", body)
	}
	if again.Clone().OnboardingDone != true {
		t.Fatal("clone dropped onboarding_done")
	}

	cleared, err := storage.Update(func(cfg *Config) error {
		if !cfg.OnboardingDone {
			t.Fatal("stored true was lost before the replay")
		}
		cfg.OnboardingDone = false
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.OnboardingDone {
		t.Fatal("clearing the tour mark did not stick")
	}
	reloaded, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.OnboardingDone {
		t.Fatal("onboarding_done stayed true after it was cleared")
	}
	body, err = os.ReadFile(storage.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "onboarding_done = true") {
		t.Fatalf("cleared tour mark was still true:\n%s", body)
	}
}
