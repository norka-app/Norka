package main

import (
	"errors"
	"fmt"

	"github.com/norka-app/Norka/internal/biz"
	"github.com/norka-app/Norka/internal/features"
	"github.com/norka-app/Norka/internal/model"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) profileSnapshot() ([]model.Profile, int, bool, error) {
	if err := a.ensureReady(); err != nil {
		return nil, 0, false, err
	}
	cfg, err := a.storage.Load()
	if err != nil {
		return nil, 0, false, err
	}
	profiles := append([]model.Profile{}, cfg.Profiles...)
	if profiles == nil {
		profiles = []model.Profile{}
	}
	return profiles, cfg.ActiveProfileID, cfg.ProfileStopOthers, nil
}

func (a *App) CreateProfile(payload model.ProfilePayload) (model.Profile, error) {
	if err := a.ensureReady(); err != nil {
		return model.Profile{}, err
	}
	created, err := a.profile.Create(payload)
	if err != nil {
		return model.Profile{}, err
	}
	a.invalidateTrayMenu()
	return created, nil
}

func (a *App) UpdateProfile(id int, payload model.ProfilePayload) (model.Profile, error) {
	if err := a.ensureReady(); err != nil {
		return model.Profile{}, err
	}
	updated, err := a.profile.Update(id, payload)
	if err != nil {
		return model.Profile{}, err
	}
	a.invalidateTrayMenu()
	return updated, nil
}

func (a *App) DeleteProfile(id int) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	if err := a.profile.Delete(id); err != nil {
		return err
	}
	a.invalidateTrayMenu()
	return nil
}

func (a *App) SetProfileStopOthers(enabled bool) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	return a.profile.SetStopOthers(enabled)
}

func (a *App) ClearActiveProfile() error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	if err := a.profile.ClearActive(); err != nil {
		return err
	}
	a.invalidateTrayMenu()
	return nil
}

// ActivateProfile connects every tunnel in the profile. Tunnels outside it
// keep running unless the saved «остановить остальные» preference is on.
// Port conflicts are returned and those tunnels are not started.
func (a *App) ActivateProfile(id int) (model.ProfileActivationResult, error) {
	if err := a.ensureReady(); err != nil {
		return model.ProfileActivationResult{}, err
	}
	if !a.featureOn(features.Profiles) {
		return model.ProfileActivationResult{}, fmt.Errorf("profiles are disabled")
	}
	result, err := a.profile.Activate(id, func(tunnelID int) error {
		_, stopErr := a.tunnel.Stop(tunnelID)
		return stopErr
	}, func(tunnelID int) error {
		updated, startErr := a.tunnel.Toggle(tunnelID, a.tunnelStartLimit())
		if startErr != nil {
			return startErr
		}
		if updated.Status == "error" {
			if updated.LastError != "" {
				return errors.New(updated.LastError)
			}
			return errors.New("start failed")
		}
		return nil
	})
	a.afterTrayAction()
	return result, err
}

func (a *App) trayActivateProfile(id int) {
	if id == 0 {
		if err := a.ClearActiveProfile(); err != nil {
			return
		}
		a.afterTrayAction()
		return
	}
	if _, err := a.ActivateProfile(id); err != nil && !errors.Is(err, biz.ErrProfileNotFound) {
		return
	}
}

func (a *App) trayShowProfiles(showWindow func()) {
	if a.ctx == nil || !a.featureOn(features.Profiles) {
		return
	}
	showWindow()
	wailsruntime.EventsEmit(a.ctx, eventWindowPage, "profiles")
}
