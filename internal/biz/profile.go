package biz

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"norka/internal/conf"
	"norka/internal/model"
)

const (
	maxProfileNameRunes  = 40
	maxProfileEmojiRunes = 8
)

var (
	ErrProfileNotFound   = errors.New("profile not found")
	ErrProfileNameExists = errors.New("profile name already exists")
)

var profileColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ProfileBiz stores named tunnel sets. Starting tunnels is done by the caller
// via Activate, which plans first and then asks TunnelBiz to stop and start.
type ProfileBiz struct {
	storage *conf.Storage
}

func NewProfileBiz(storage *conf.Storage) *ProfileBiz {
	return &ProfileBiz{storage: storage}
}

func (b *ProfileBiz) List() ([]model.Profile, error) {
	cfg, err := b.storage.Load()
	if err != nil {
		return nil, err
	}
	return append([]model.Profile{}, cfg.Profiles...), nil
}

func (b *ProfileBiz) Create(payload model.ProfilePayload) (model.Profile, error) {
	var created model.Profile
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		normalized, err := normalizeProfilePayload(payload, cfg.Tunnels)
		if err != nil {
			return err
		}
		if profileNameTaken(cfg.Profiles, normalized.Name, 0) {
			return ErrProfileNameExists
		}
		created = model.Profile{
			ID:        nextProfileID(cfg.Profiles),
			Name:      normalized.Name,
			Color:     normalized.Color,
			Emoji:     normalized.Emoji,
			TunnelIDs: normalized.TunnelIDs,
		}
		cfg.Profiles = append(cfg.Profiles, created)
		return nil
	})
	if err != nil {
		return model.Profile{}, err
	}
	return created, nil
}

func (b *ProfileBiz) Update(id int, payload model.ProfilePayload) (model.Profile, error) {
	if id <= 0 {
		return model.Profile{}, fmt.Errorf("invalid profile id")
	}
	var updated model.Profile
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		idx := profileIndex(cfg.Profiles, id)
		if idx < 0 {
			return ErrProfileNotFound
		}
		normalized, err := normalizeProfilePayload(payload, cfg.Tunnels)
		if err != nil {
			return err
		}
		if profileNameTaken(cfg.Profiles, normalized.Name, id) {
			return ErrProfileNameExists
		}
		updated = model.Profile{
			ID:        id,
			Name:      normalized.Name,
			Color:     normalized.Color,
			Emoji:     normalized.Emoji,
			TunnelIDs: normalized.TunnelIDs,
		}
		cfg.Profiles[idx] = updated
		return nil
	})
	if err != nil {
		return model.Profile{}, err
	}
	return updated, nil
}

func (b *ProfileBiz) Delete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid profile id")
	}
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		idx := profileIndex(cfg.Profiles, id)
		if idx < 0 {
			return ErrProfileNotFound
		}
		cfg.Profiles = append(cfg.Profiles[:idx], cfg.Profiles[idx+1:]...)
		if cfg.ActiveProfileID == id {
			cfg.ActiveProfileID = 0
		}
		return nil
	})
	return err
}

func (b *ProfileBiz) SetStopOthers(enabled bool) error {
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		cfg.ProfileStopOthers = enabled
		return nil
	})
	return err
}

func (b *ProfileBiz) ClearActive() error {
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		cfg.ActiveProfileID = 0
		return nil
	})
	return err
}

// Activate connects the profile's tunnels. stop and start are invoked in plan
// order (stops first). The profile becomes the active one even if some tunnels
// fail; the result lists conflicts and errors for the UI.
func (b *ProfileBiz) Activate(id int, stop func(int) error, start func(int) error) (model.ProfileActivationResult, error) {
	if id <= 0 {
		return model.ProfileActivationResult{}, fmt.Errorf("invalid profile id")
	}
	cfg, err := b.storage.Load()
	if err != nil {
		return model.ProfileActivationResult{}, err
	}
	idx := profileIndex(cfg.Profiles, id)
	if idx < 0 {
		return model.ProfileActivationResult{}, ErrProfileNotFound
	}
	plan := PlanProfileActivation(cfg.Profiles[idx], cfg.Tunnels, cfg.ProfileStopOthers)
	result := ApplyActivationPlan(plan, stop, start)
	result.ProfileID = id
	if _, err := b.storage.Update(func(cfg *conf.Config) error {
		if profileIndex(cfg.Profiles, id) < 0 {
			return ErrProfileNotFound
		}
		cfg.ActiveProfileID = id
		return nil
	}); err != nil {
		return result, err
	}
	return result, nil
}

// detachTunnelFromProfiles drops a deleted tunnel from every profile.
func detachTunnelFromProfiles(cfg *conf.Config, tunnelID int) {
	if cfg == nil || tunnelID <= 0 {
		return
	}
	for i := range cfg.Profiles {
		cfg.Profiles[i].TunnelIDs = removeInt(cfg.Profiles[i].TunnelIDs, tunnelID)
	}
}

func normalizeProfilePayload(payload model.ProfilePayload, tunnels []model.Tunnel) (model.ProfilePayload, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Emoji = strings.TrimSpace(payload.Emoji)
	payload.Color = strings.TrimSpace(payload.Color)
	if payload.Name == "" {
		return payload, fmt.Errorf("name is required")
	}
	if utf8.RuneCountInString(payload.Name) > maxProfileNameRunes {
		return payload, fmt.Errorf("name is too long")
	}
	if utf8.RuneCountInString(payload.Emoji) > maxProfileEmojiRunes {
		return payload, fmt.Errorf("emoji is too long")
	}
	if payload.Color != "" && !profileColorPattern.MatchString(payload.Color) {
		return payload, fmt.Errorf("color must be #RRGGBB")
	}
	known := make(map[int]struct{}, len(tunnels))
	for _, tunnel := range tunnels {
		if tunnel.ID > 0 {
			known[tunnel.ID] = struct{}{}
		}
	}
	ids := make([]int, 0, len(payload.TunnelIDs))
	seen := make(map[int]struct{}, len(payload.TunnelIDs))
	for _, id := range payload.TunnelIDs {
		if id <= 0 {
			continue
		}
		if _, ok := known[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	payload.TunnelIDs = ids
	return payload, nil
}

func profileNameTaken(profiles []model.Profile, name string, excludeID int) bool {
	normalized := strings.TrimSpace(name)
	for _, profile := range profiles {
		if excludeID > 0 && profile.ID == excludeID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(profile.Name), normalized) {
			return true
		}
	}
	return false
}

func profileIndex(profiles []model.Profile, id int) int {
	for i := range profiles {
		if profiles[i].ID == id {
			return i
		}
	}
	return -1
}

func nextProfileID(profiles []model.Profile) int {
	next := 1
	for _, profile := range profiles {
		if profile.ID >= next {
			next = profile.ID + 1
		}
	}
	return next
}

func removeInt(ids []int, drop int) []int {
	if len(ids) == 0 {
		return ids
	}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}
