package biz

import (
	"errors"
	"fmt"
	"strings"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/forward"
	"github.com/norka-app/Norka/internal/model"
	"github.com/norka-app/Norka/internal/secrets"
)

var (
	ErrJumperNotFound = errors.New("jumper not found")
	ErrJumperInUse    = errors.New("jumper is used by existing tunnels")
)

const (
	defaultKeepAliveIntervalMs = 5000
	minKeepAliveIntervalMs     = 1000
	maxKeepAliveIntervalMs     = 120000
)

type JumperBiz struct {
	storage *conf.Storage
	secrets *secrets.Vault
}

func NewJumperBiz(storage *conf.Storage) *JumperBiz {
	return &JumperBiz{storage: storage}
}

// SetSecrets attaches the OS keychain. Without it, passwords stay in config.toml.
func (b *JumperBiz) SetSecrets(vault *secrets.Vault) {
	if b == nil {
		return
	}
	b.secrets = vault
}

func (b *JumperBiz) List() ([]model.Jumper, error) {
	cfg, err := b.storage.Load()
	if err != nil {
		return nil, err
	}

	items := append([]model.Jumper{}, cfg.Jumpers...)
	if b.secrets != nil {
		for i := range items {
			secrets.Redact(&items[i])
		}
	}
	return items, nil
}

func (b *JumperBiz) Create(payload model.JumperPayload) (model.Jumper, error) {
	payload = normalizeJumperPayload(payload)
	if payload.Password == "" && payload.SecretSourceID > 0 {
		if err := b.copySecretIntoPayload(&payload, payload.SecretSourceID); err != nil {
			return model.Jumper{}, err
		}
	}
	if err := validateJumperPayload(payload, false); err != nil {
		return model.Jumper{}, err
	}

	var created model.Jumper
	var forgetRef string
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		created = jumperFromPayload(nextJumperID(cfg.Jumpers), payload)
		forgetRef = b.sealJumper(nil, &created, payload.Password)
		cfg.Jumpers = append(cfg.Jumpers, created)
		return nil
	})
	if err != nil {
		return model.Jumper{}, err
	}
	if b.secrets != nil {
		b.secrets.Forget(forgetRef)
		secrets.Redact(&created)
	}

	return created, nil
}

func (b *JumperBiz) Update(id int, payload model.JumperPayload) (model.Jumper, error) {
	if id <= 0 {
		return model.Jumper{}, fmt.Errorf("invalid jumper id")
	}

	payload = normalizeJumperPayload(payload)
	existing, found, err := b.loadJumper(id)
	if err != nil {
		return model.Jumper{}, err
	}
	if !found {
		return model.Jumper{}, ErrJumperNotFound
	}
	if err := validateJumperPayload(payload, jumperHasSecret(existing)); err != nil {
		return model.Jumper{}, err
	}

	var updated model.Jumper
	var forgetRef string
	_, err = b.storage.Update(func(cfg *conf.Config) error {
		idx := -1
		for i := range cfg.Jumpers {
			if cfg.Jumpers[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrJumperNotFound
		}

		current := cfg.Jumpers[idx]
		updated = jumperFromPayload(id, payload)
		forgetRef = b.sealJumper(&current, &updated, payload.Password)
		cfg.Jumpers[idx] = updated
		return nil
	})
	if err != nil {
		return model.Jumper{}, err
	}
	if b.secrets != nil {
		b.secrets.Forget(forgetRef)
		secrets.Redact(&updated)
	}

	return updated, nil
}

func (b *JumperBiz) Delete(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid jumper id")
	}

	var forgetRef string
	_, err := b.storage.Update(func(cfg *conf.Config) error {
		for _, tunnel := range cfg.Tunnels {
			for _, jid := range tunnel.JumperIDs {
				if jid == id {
					return ErrJumperInUse
				}
			}
		}

		idx := -1
		for i := range cfg.Jumpers {
			if cfg.Jumpers[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrJumperNotFound
		}

		forgetRef = cfg.Jumpers[idx].SecretRef
		cfg.Jumpers = append(cfg.Jumpers[:idx], cfg.Jumpers[idx+1:]...)
		return nil
	})
	if err != nil {
		return err
	}
	if b.secrets != nil {
		b.secrets.Forget(forgetRef)
	}
	return nil
}

func (b *JumperBiz) TestConnection(payload model.JumperPayload) error {
	payload = normalizeJumperPayload(payload)
	if payload.Password == "" {
		sourceID := payload.ID
		if sourceID <= 0 {
			sourceID = payload.SecretSourceID
		}
		if sourceID > 0 {
			if err := b.copySecretIntoPayload(&payload, sourceID); err != nil {
				return err
			}
		}
	}
	if err := validateJumperPayload(payload, false); err != nil {
		return err
	}

	j := model.Jumper{
		Name:                   payload.Name,
		Host:                   payload.Host,
		Port:                   payload.Port,
		User:                   payload.User,
		AuthType:               payload.AuthType,
		KeyPath:                payload.KeyPath,
		AgentSocketPath:        payload.AgentSocketPath,
		Password:               payload.Password,
		BypassHostVerification: payload.BypassHostVerification,
		KeepAliveIntervalMs:    payload.KeepAliveIntervalMs,
		TimeoutMs:              payload.TimeoutMs,
		HostKeyAlgorithms:      payload.HostKeyAlgorithms,
		Notes:                  payload.Notes,
	}

	return forward.TestJumperConnection(j)
}

func normalizeJumperPayload(payload model.JumperPayload) model.JumperPayload {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Host = strings.TrimSpace(payload.Host)
	payload.User = strings.TrimSpace(payload.User)
	payload.AuthType = strings.TrimSpace(payload.AuthType)
	payload.KeyPath = strings.TrimSpace(payload.KeyPath)
	payload.AgentSocketPath = strings.TrimSpace(payload.AgentSocketPath)
	payload.HostKeyAlgorithms = strings.TrimSpace(payload.HostKeyAlgorithms)
	payload.Notes = strings.TrimSpace(payload.Notes)

	if payload.Port <= 0 {
		payload.Port = 22
	}
	if payload.TimeoutMs <= 0 {
		payload.TimeoutMs = 5000
	}
	if payload.KeepAliveIntervalMs < 0 {
		payload.KeepAliveIntervalMs = defaultKeepAliveIntervalMs
	}
	if payload.AuthType == "" {
		payload.AuthType = "ssh_key"
	}
	if payload.AuthType != "ssh_key" {
		payload.KeyPath = ""
	}
	if payload.AuthType == "ssh_agent" {
		payload.Password = ""
	}

	return payload
}

func validateJumperPayload(payload model.JumperPayload, hasStoredSecret bool) error {
	if payload.Name == "" {
		return fmt.Errorf("name is required")
	}
	if payload.Host == "" {
		return fmt.Errorf("host is required")
	}
	if payload.User == "" {
		return fmt.Errorf("user is required")
	}
	if payload.Port < 1 || payload.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if payload.TimeoutMs < 100 || payload.TimeoutMs > 120000 {
		return fmt.Errorf("timeoutMs must be between 100 and 120000")
	}
	if payload.KeepAliveIntervalMs > maxKeepAliveIntervalMs {
		return fmt.Errorf("keepAliveIntervalMs must be 0 (disable) or between %d and %d", minKeepAliveIntervalMs, maxKeepAliveIntervalMs)
	}
	if payload.KeepAliveIntervalMs > 0 && payload.KeepAliveIntervalMs < minKeepAliveIntervalMs {
		return fmt.Errorf("keepAliveIntervalMs must be 0 (disable) or between %d and %d", minKeepAliveIntervalMs, maxKeepAliveIntervalMs)
	}
	switch payload.AuthType {
	case "password":
		if payload.Password == "" && !hasStoredSecret {
			return fmt.Errorf("password auth requires password")
		}
	case "ssh_key":
		if payload.KeyPath == "" {
			return fmt.Errorf("ssh_key auth requires keyPath")
		}
	case "ssh_agent":
	default:
		return fmt.Errorf("unsupported authType: %s", payload.AuthType)
	}
	return nil
}

func jumperFromPayload(id int, payload model.JumperPayload) model.Jumper {
	return model.Jumper{
		ID:                     id,
		Name:                   payload.Name,
		Host:                   payload.Host,
		Port:                   payload.Port,
		User:                   payload.User,
		AuthType:               payload.AuthType,
		KeyPath:                payload.KeyPath,
		AgentSocketPath:        payload.AgentSocketPath,
		BypassHostVerification: payload.BypassHostVerification,
		KeepAliveIntervalMs:    payload.KeepAliveIntervalMs,
		TimeoutMs:              payload.TimeoutMs,
		HostKeyAlgorithms:      payload.HostKeyAlgorithms,
		Notes:                  payload.Notes,
	}
}

func jumperHasSecret(jumper model.Jumper) bool {
	return jumper.Password != "" || strings.TrimSpace(jumper.SecretRef) != ""
}

func (b *JumperBiz) sealJumper(existing *model.Jumper, next *model.Jumper, plaintext string) string {
	if b == nil || b.secrets == nil {
		next.Password = plaintext
		if existing != nil && plaintext == "" && next.AuthType != "ssh_agent" {
			next.Password = existing.Password
			next.SecretRef = existing.SecretRef
		}
		if next.AuthType == "ssh_agent" {
			next.Password = ""
			next.SecretRef = ""
		}
		return ""
	}
	return b.secrets.Seal(existing, next, plaintext)
}

func (b *JumperBiz) copySecretIntoPayload(payload *model.JumperPayload, id int) error {
	jumper, found, err := b.loadJumper(id)
	if err != nil {
		return err
	}
	if !found {
		return ErrJumperNotFound
	}
	if b.secrets != nil {
		b.secrets.Open(&jumper)
	}
	payload.Password = jumper.Password
	return nil
}

func (b *JumperBiz) loadJumper(id int) (model.Jumper, bool, error) {
	cfg, err := b.storage.Load()
	if err != nil {
		return model.Jumper{}, false, err
	}
	for _, jumper := range cfg.Jumpers {
		if jumper.ID == id {
			return jumper, true, nil
		}
	}
	return model.Jumper{}, false, nil
}

func nextJumperID(items []model.Jumper) int {
	next := 1
	for _, item := range items {
		if item.ID >= next {
			next = item.ID + 1
		}
	}
	return next
}
