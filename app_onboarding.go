package main

import "github.com/norka-app/Norka/internal/conf"

// GetOnboardingDone reports whether the first-run tour was finished or skipped.
func (a *App) GetOnboardingDone() (bool, error) {
	if err := a.ensureReady(); err != nil {
		return false, err
	}
	cfg, err := a.storage().Load()
	if err != nil {
		return false, err
	}
	return cfg.OnboardingDone, nil
}

// SetOnboardingDone stores the tour mark. Turning the onboarding flag off does
// not call this, and this does not delete tunnels or other settings.
func (a *App) SetOnboardingDone(done bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	_, err := a.storage().Update(func(cfg *conf.Config) error {
		cfg.OnboardingDone = done
		return nil
	})
	return err
}
