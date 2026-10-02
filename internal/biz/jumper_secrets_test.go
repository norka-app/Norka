package biz

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"norka/internal/conf"
	"norka/internal/model"
	"norka/internal/secrets"
)

func TestJumperSecretRoundTripAndDelete(t *testing.T) {
	dir := t.TempDir()
	storage, err := conf.NewStorage(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	ring := secrets.NewMemoryKeyring()
	vault := secrets.NewVault(ring, true)
	jumpers := NewJumperBiz(storage)
	jumpers.SetSecrets(vault)

	created, err := jumpers.Create(model.JumperPayload{
		Name:     "db",
		Host:     "example.test",
		Port:     22,
		User:     "root",
		AuthType: "password",
		Password: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Password != "" || !created.HasSecret {
		t.Fatalf("create response leaked or missed secret: %+v", created)
	}

	raw, err := os.ReadFile(storage.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret") {
		t.Fatalf("config contains plaintext: %s", raw)
	}
	cfg, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	ref := cfg.Jumpers[0].SecretRef
	got, err := ring.Get(secrets.ServiceName, ref)
	if err != nil || got != "s3cret" {
		t.Fatalf("keychain = %q %v", got, err)
	}

	updated, err := jumpers.Update(created.ID, model.JumperPayload{
		Name:     "db",
		Host:     "example.test",
		Port:     22,
		User:     "root",
		AuthType: "password",
		Password: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HasSecret {
		t.Fatal("empty password cleared the stored secret")
	}
	got, err = ring.Get(secrets.ServiceName, ref)
	if err != nil || got != "s3cret" {
		t.Fatalf("kept keychain = %q %v", got, err)
	}

	listed, err := jumpers.List()
	if err != nil {
		t.Fatal(err)
	}
	if listed[0].Password != "" || !listed[0].HasSecret {
		t.Fatalf("list = %+v", listed[0])
	}

	copied, err := jumpers.Create(model.JumperPayload{
		Name:           "db-copy",
		Host:           "example.test",
		Port:           22,
		User:           "root",
		AuthType:       "password",
		SecretSourceID: created.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !copied.HasSecret {
		t.Fatal("copy did not keep a secret")
	}

	if err := jumpers.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ring.Get(secrets.ServiceName, ref); err == nil {
		t.Fatal("deleted jumper secret is still in the keychain")
	}
}
