package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"norka/internal/conf"
	"norka/internal/model"
)

func TestMigratePlaintextSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.Update(func(cfg *conf.Config) error {
		cfg.Jumpers = []model.Jumper{
			{ID: 1, Name: "db", Host: "example", Password: "s3cret"},
			{ID: 2, Name: "plain", Host: "example", Password: ""},
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ring := NewMemoryKeyring()
	vault := NewVault(ring, true)
	result, err := vault.MigrateStorage(storage)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v", result)
	}
	if result.BackupPath == "" {
		t.Fatal("backup path is empty")
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), `password = "s3cret"`) {
		t.Fatalf("backup missing plaintext secret: %s", backup)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "s3cret") {
		t.Fatalf("config still contains plaintext: %s", saved)
	}
	if !strings.Contains(string(saved), "secret_ref") {
		t.Fatalf("config missing secret_ref: %s", saved)
	}

	cfg, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Jumpers[0].Password != "" || cfg.Jumpers[0].SecretRef == "" {
		t.Fatalf("jumper after migration = %+v", cfg.Jumpers[0])
	}
	got, err := ring.Get(ServiceName, cfg.Jumpers[0].SecretRef)
	if err != nil || got != "s3cret" {
		t.Fatalf("keychain = %q, %v", got, err)
	}
}

func TestMigrateSkipsUnavailableKeychain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.Update(func(cfg *conf.Config) error {
		cfg.Jumpers = []model.Jumper{{ID: 1, Name: "db", Password: "s3cret"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	vault := NewVault(NewMemoryKeyring(), false)
	result, err := vault.MigrateStorage(storage)
	if err != nil {
		t.Fatal(err)
	}
	if result.KeychainAvailable || result.Migrated != 0 || result.BackupPath != "" {
		t.Fatalf("result = %+v", result)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "s3cret") {
		t.Fatalf("plaintext should stay in config: %s", saved)
	}
	matches, _ := filepath.Glob(path + ".pre-keyring-*")
	if len(matches) != 0 {
		t.Fatalf("unexpected backup: %v", matches)
	}
}

func TestMigratePartialFailureKeepsPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.Update(func(cfg *conf.Config) error {
		cfg.Jumpers = []model.Jumper{
			{ID: 1, Name: "ok", Password: "good"},
			{ID: 2, Name: "bad", Password: "bad"},
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ring := NewMemoryKeyring()
	ring.FailIfSecret("bad", errors.New("keychain full"))
	vault := NewVault(ring, true)
	result, err := vault.MigrateStorage(storage)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 || result.Failed != 1 {
		t.Fatalf("result = %+v", result)
	}
	cfg, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Jumpers[0].Password != "" || cfg.Jumpers[0].SecretRef == "" {
		t.Fatalf("successful jumper = %+v", cfg.Jumpers[0])
	}
	if cfg.Jumpers[1].Password != "bad" || cfg.Jumpers[1].SecretRef != "" {
		t.Fatalf("failed jumper = %+v", cfg.Jumpers[1])
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	storage, err := conf.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.Update(func(cfg *conf.Config) error {
		cfg.Jumpers = []model.Jumper{{ID: 7, Name: "db", Host: "example", User: "root", Password: "s3cret"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ring := NewMemoryKeyring()
	vault := NewVault(ring, true)
	if _, err := vault.MigrateStorage(storage); err != nil {
		t.Fatal(err)
	}
	live, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}

	redacted := PrepareExport(live, vault, false)
	redactedTOML := string(conf.MarshalTOML(redacted))
	if strings.Contains(redactedTOML, "s3cret") {
		t.Fatalf("default export contains secret: %s", redactedTOML)
	}
	if !strings.Contains(redactedTOML, "secret_ref") {
		t.Fatalf("default export dropped keychain reference: %s", redactedTOML)
	}

	withSecrets := PrepareExport(live, vault, true)
	secretTOML := string(conf.MarshalTOML(withSecrets))
	if !strings.Contains(secretTOML, "s3cret") {
		t.Fatalf("export with passwords missing secret: %s", secretTOML)
	}

	parsed, err := conf.ParseConfigTOML([]byte(redactedTOML))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Jumpers[0].Password != "" {
		t.Fatalf("parsed redacted password = %q", parsed.Jumpers[0].Password)
	}
	vault.Open(&parsed.Jumpers[0])
	if parsed.Jumpers[0].Password != "s3cret" {
		t.Fatalf("opened password = %q", parsed.Jumpers[0].Password)
	}

	importDir := t.TempDir()
	importPath := filepath.Join(importDir, "config.toml")
	importStorage, err := conf.NewStorage(importPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(importPath, []byte(secretTOML), conf.PrivateFilePerm); err != nil {
		t.Fatal(err)
	}
	importRing := NewMemoryKeyring()
	importVault := NewVault(importRing, true)
	result, err := importVault.MigrateStorage(importStorage)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 {
		t.Fatalf("import migration = %+v", result)
	}
	imported, err := importStorage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if imported.Jumpers[0].Password != "" || imported.Jumpers[0].SecretRef == "" {
		t.Fatalf("imported jumper = %+v", imported.Jumpers[0])
	}
	got, err := importRing.Get(ServiceName, imported.Jumpers[0].SecretRef)
	if err != nil || got != "s3cret" {
		t.Fatalf("imported keychain = %q, %v", got, err)
	}
	saved, err := os.ReadFile(importPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "s3cret") {
		t.Fatalf("imported config still has plaintext: %s", saved)
	}
}
