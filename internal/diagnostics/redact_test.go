package diagnostics

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/features"
)

const secretCanary = "super-secret-canary-value"

// secretCases is every string field that can hold a secret. The reflection
// guard below fails when a classified secret is missing from this table.
var secretCases = []struct {
	name  string
	field string
	set   func(*conf.Config, string)
}{
	{name: "jumper password", field: "Jumper.Password", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].Password = v }},
	{name: "jumper key path", field: "Jumper.KeyPath", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].KeyPath = v }},
	{name: "jumper agent socket", field: "Jumper.AgentSocketPath", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].AgentSocketPath = v }},
	{name: "jumper keychain ref", field: "Jumper.SecretRef", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].SecretRef = v }},
	{name: "jumper notes", field: "Jumper.Notes", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].Notes = v }},
	{name: "payload password", field: "JumperPayload.Password", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].Password = v }},
	{name: "payload key path", field: "JumperPayload.KeyPath", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].KeyPath = v }},
	{name: "payload agent socket", field: "JumperPayload.AgentSocketPath", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].AgentSocketPath = v }},
	{name: "payload notes", field: "JumperPayload.Notes", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].Notes = v }},
}

func TestSecretFieldsRedacted(t *testing.T) {
	for _, tc := range secretCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := sampleConfig()
			tc.set(cfg, secretCanary)
			text := archiveText(t, cfg, false, nil)
			if strings.Contains(text, secretCanary) {
				t.Fatalf("%s leaked into the archive", tc.field)
			}
			if !strings.Contains(text, Redacted) {
				t.Fatalf("%s was dropped instead of replaced with %s", tc.field, Redacted)
			}
		})
	}
}

func TestSecretTableCoversEverySecretClass(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range secretCases {
		if seen[tc.field] {
			t.Fatalf("duplicate secret case %s", tc.field)
		}
		seen[tc.field] = true
		if fieldClass[tc.field] != classSecret {
			t.Fatalf("%s is in the secret table but classified as %q", tc.field, fieldClass[tc.field])
		}
	}
	for key, kind := range fieldClass {
		if kind == classSecret && !seen[key] {
			t.Errorf("secret field %s is missing from the redaction table", key)
		}
	}
}

func TestSSHConfigStringFieldsAreClassified(t *testing.T) {
	known := map[string]reflect.Type{}
	for _, typ := range sshConfigTypes() {
		known[typ.Name()] = typ
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" || field.Type.Kind() != reflect.String {
				continue
			}
			key := typ.Name() + "." + field.Name
			if _, ok := fieldClass[key]; !ok {
				t.Errorf("new string field %s is not classified", key)
			}
		}
	}
	for key := range fieldClass {
		structName, fieldName, ok := strings.Cut(key, ".")
		if !ok {
			t.Errorf("classification key %s is not Struct.Field", key)
			continue
		}
		typ, exists := known[structName]
		if !exists {
			t.Errorf("classified %s but %s is not an SSH or tunnel config struct", key, structName)
			continue
		}
		field, exists := typ.FieldByName(fieldName)
		if !exists || field.Type.Kind() != reflect.String {
			t.Errorf("classified %s but that string field does not exist", key)
		}
	}
}

func TestSensitiveFieldNamesAreSecrets(t *testing.T) {
	suffixes := []string{"Password", "Passphrase", "Secret", "SecretRef", "Token", "KeyPath", "PrivateKey", "AgentSocketPath"}
	for _, typ := range sshConfigTypes() {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Type.Kind() != reflect.String {
				continue
			}
			sensitive := false
			for _, suffix := range suffixes {
				if strings.HasSuffix(field.Name, suffix) {
					sensitive = true
					break
				}
			}
			if !sensitive {
				continue
			}
			key := typ.Name() + "." + field.Name
			if fieldClass[key] != classSecret {
				t.Errorf("%s can hold a secret and must be classified as secret", key)
			}
		}
	}
}

func TestHostsAndUsersRedactedUnlessOptIn(t *testing.T) {
	cases := []struct {
		name  string
		value string
		set   func(*conf.Config, string)
	}{
		{name: "jumper host", value: "db.internal", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].Host = v }},
		{name: "jumper user", value: "alice", set: func(cfg *conf.Config, v string) { cfg.Jumpers[0].User = v }},
		{name: "local host", value: "127.0.0.1", set: func(cfg *conf.Config, v string) { cfg.Tunnels[0].LocalHost = v }},
		{name: "remote host", value: "10.1.2.3", set: func(cfg *conf.Config, v string) { cfg.Tunnels[0].RemoteHost = v }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := sampleConfig()
			tc.set(cfg, tc.value)
			cfg.Jumpers[0].Password = secretCanary
			hidden := archiveText(t, cfg, false, nil)
			if strings.Contains(hidden, tc.value) || strings.Contains(hidden, secretCanary) {
				t.Fatalf("opt-out archive still has %q or the password", tc.value)
			}
			shown := archiveText(t, cfg, true, nil)
			if !strings.Contains(shown, tc.value) {
				t.Fatalf("opt-in archive dropped %q", tc.value)
			}
			if strings.Contains(shown, secretCanary) {
				t.Fatal("opt-in archive kept the password")
			}
		})
	}
}

func TestEmbeddedSecretsAndAddressesScrubbed(t *testing.T) {
	cfg := sampleConfig()
	cfg.Jumpers[0].Password = secretCanary
	cfg.Jumpers[0].Host = "db.internal"
	cfg.Jumpers[0].User = "alice"
	cfg.Groups[0].Name = "group-" + secretCanary
	cfg.Profiles[0].Name = "profile-db.internal"
	cfg.Tunnels[0].LastError = "dial db.internal as alice failed: " + secretCanary
	cfg.Tunnels[0].Description = "uses " + secretCanary
	log := []byte("connected db.internal user alice token follows\n" + secretCanary + "\n")
	text := archiveText(t, cfg, false, log, secretCanary)
	for _, leaked := range []string{secretCanary, "db.internal", "alice"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("archive still contains %q", leaked)
		}
	}
	if !strings.Contains(fileText(t, cfg, false, nil, "config.toml"), "local_port = 5432") {
		t.Fatal("local port was removed with the address")
	}
	if !strings.Contains(fileText(t, cfg, false, nil, "config.toml"), `mode = "local"`) &&
		!strings.Contains(fileText(t, cfg, false, nil, "config.toml"), "mode = 'local'") {
		body := fileText(t, cfg, false, nil, "config.toml")
		if !strings.Contains(body, "local") {
			t.Fatalf("mode was removed:\n%s", body)
		}
	}
}

func TestKeyFileContentsAreNeverRead(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	const material = "CANARY-KEY-MATERIAL-9f3a"
	body := "-----BEGIN OPENSSH PRIVATE KEY-----\n" + material + "\n-----END OPENSSH PRIVATE KEY-----\n"
	if err := os.WriteFile(keyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := sampleConfig()
	cfg.Jumpers[0].KeyPath = keyPath
	cfg.Jumpers[0].Password = "passphrase-canary-zz"
	log := []byte("paste follows\n" + body)
	text := archiveText(t, cfg, true, log)
	for _, leaked := range []string{material, keyPath, "passphrase-canary-zz", "BEGIN OPENSSH PRIVATE KEY"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("archive contains %q", leaked)
		}
	}
}

func TestAutomationTokenIsNotIncluded(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg := sampleConfig()
	log := []byte("automation token " + token + " accepted\n")
	text := archiveText(t, cfg, true, log, token)
	if strings.Contains(text, token) {
		t.Fatal("automation token was written into the archive")
	}
	for _, file := range mustBundle(t, cfg, true, log, token).Files {
		if file.Name == "automation.token" {
			t.Fatal("token file was added to the archive")
		}
	}
}

func TestStructurePortsModesAndFlagsStay(t *testing.T) {
	cfg := sampleConfig()
	cfg.Jumpers[0].Password = secretCanary
	cfg.Jumpers[0].Host = "db.internal"
	cfg.Jumpers[0].AuthType = "ssh_key"
	cfg.Tunnels[0].Mode = "local"
	cfg.Tunnels[0].LocalPort = 5432
	cfg.Tunnels[0].RemotePort = 5432
	if err := cfg.Features.Set(features.Diagnostics, false); err != nil {
		t.Fatal(err)
	}
	body := fileText(t, cfg, false, nil, "config.toml")
	for _, want := range []string{"5432", "ssh_key", "local"} {
		if !strings.Contains(body, want) {
			t.Fatalf("config lost %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, secretCanary) || strings.Contains(body, "db.internal") {
		t.Fatalf("config kept a secret or a host:\n%s", body)
	}
	if !strings.Contains(body, "diagnostics = false") {
		t.Fatalf("explicit flag was dropped:\n%s", body)
	}
}
