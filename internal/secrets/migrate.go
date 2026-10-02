package secrets

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"norka/internal/conf"
	"norka/internal/model"
)

// MigrateResult describes a startup move of plaintext jumper secrets.
type MigrateResult struct {
	KeychainAvailable bool
	Migrated          int
	Failed            int
	BackupPath        string
}

// MigrateStorage moves plaintext jumper passwords into the keychain and strips
// them from config.toml. The previous file is copied aside first.
func (v *Vault) MigrateStorage(storage *conf.Storage) (MigrateResult, error) {
	result := MigrateResult{KeychainAvailable: v.Available()}
	if storage == nil || !result.KeychainAvailable {
		return result, nil
	}

	cfg, err := storage.Load()
	if err != nil {
		return result, err
	}
	if !hasPlaintextSecret(cfg) {
		return result, nil
	}

	backupPath, err := backupConfig(storage.Path())
	if err != nil {
		slog.Error("jumper secret migration aborted; config backup failed", "error", err)
		return result, err
	}
	result.BackupPath = backupPath

	_, err = storage.Update(func(cfg *conf.Config) error {
		for i := range cfg.Jumpers {
			password := cfg.Jumpers[i].Password
			if password == "" {
				continue
			}
			ref := cfg.Jumpers[i].SecretRef
			if ref == "" {
				ref = newSecretRef()
			}
			if err := v.ring.Set(ServiceName, ref, password); err != nil {
				result.Failed++
				slog.Error("jumper secret was not moved to keychain", "jumper_id", cfg.Jumpers[i].ID, "name", cfg.Jumpers[i].Name, "error", err)
				continue
			}
			cfg.Jumpers[i].SecretRef = ref
			cfg.Jumpers[i].Password = ""
			result.Migrated++
			slog.Info("moved jumper secret to keychain", "jumper_id", cfg.Jumpers[i].ID, "name", cfg.Jumpers[i].Name)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	slog.Info("jumper secret migration finished", "migrated", result.Migrated, "failed", result.Failed, "backup", backupPath)
	return result, nil
}

func hasPlaintextSecret(cfg *conf.Config) bool {
	if cfg == nil {
		return false
	}
	for _, jumper := range cfg.Jumpers {
		if jumper.Password != "" {
			return true
		}
	}
	return false
}

func backupConfig(path string) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	dest := path + ".pre-keyring-" + stamp
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open config for backup: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, conf.PrivateFilePerm)
	if err != nil {
		return "", fmt.Errorf("create config backup: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return "", fmt.Errorf("copy config backup: %w", err)
	}
	if err := dst.Close(); err != nil {
		return "", err
	}
	return dest, nil
}

// SecretRefs collects keychain references stored on jumpers.
func SecretRefs(jumpers []model.Jumper) map[string]struct{} {
	out := make(map[string]struct{})
	for _, jumper := range jumpers {
		if jumper.SecretRef == "" {
			continue
		}
		out[jumper.SecretRef] = struct{}{}
	}
	return out
}

// PrepareExport clones cfg for an export file.
// Secrets are included only when includeSecrets is true.
func PrepareExport(cfg *conf.Config, vault *Vault, includeSecrets bool) *conf.Config {
	out := cfg.Clone()
	for i := range out.Jumpers {
		if includeSecrets {
			if vault != nil {
				vault.Open(&out.Jumpers[i])
			}
			continue
		}
		out.Jumpers[i].Password = ""
	}
	return out
}
