package diagnostics

import (
	"reflect"
	"sort"
	"strings"

	"github.com/norka-app/Norka/internal/conf"
	"github.com/norka-app/Norka/internal/model"
)

// Redacted replaces a secret or an identity the user did not opt to include.
const Redacted = "***"

// class says what a string field on an SSH or tunnel config may contain.
// A new string field has to be added here. The tests fail when it is not.
type class string

const (
	// classKeep is structure: names, modes, algorithms, status.
	classKeep class = "keep"
	// classSecret is always replaced: passwords, passphrases, key paths,
	// keychain refs, agent sockets, and free-text notes that can hold them.
	classSecret class = "secret"
	// classIdentity is a host or username. It stays only when the user opts in.
	classIdentity class = "identity"
	// classScrub is free text we keep after cutting secrets and, unless the
	// user opted in, hosts and usernames found inside it.
	classScrub class = "scrub"
)

// minScrubLen skips tiny substrings so a short token cannot eat unrelated text.
// The field itself is still replaced when its class says so.
const minScrubLen = 4

// fieldClass is the classification of every string field on the SSH and tunnel
// config structs. Key is "Struct.Field".
var fieldClass = map[string]class{
	"Jumper.Name":              classKeep,
	"Jumper.Host":              classIdentity,
	"Jumper.User":              classIdentity,
	"Jumper.AuthType":          classKeep,
	"Jumper.KeyPath":           classSecret,
	"Jumper.AgentSocketPath":   classSecret,
	"Jumper.Password":          classSecret,
	"Jumper.SecretRef":         classSecret,
	"Jumper.HostKeyAlgorithms": classKeep,
	"Jumper.Notes":             classSecret,

	"JumperPayload.Name":              classKeep,
	"JumperPayload.Host":              classIdentity,
	"JumperPayload.User":              classIdentity,
	"JumperPayload.AuthType":          classKeep,
	"JumperPayload.KeyPath":           classSecret,
	"JumperPayload.AgentSocketPath":   classSecret,
	"JumperPayload.Password":          classSecret,
	"JumperPayload.HostKeyAlgorithms": classKeep,
	"JumperPayload.Notes":             classSecret,

	"Tunnel.Name":        classKeep,
	"Tunnel.Mode":        classKeep,
	"Tunnel.LocalHost":   classIdentity,
	"Tunnel.RemoteHost":  classIdentity,
	"Tunnel.Status":      classKeep,
	"Tunnel.LastError":   classScrub,
	"Tunnel.Description": classScrub,

	"TunnelPayload.Name":        classKeep,
	"TunnelPayload.Mode":        classKeep,
	"TunnelPayload.LocalHost":   classIdentity,
	"TunnelPayload.RemoteHost":  classIdentity,
	"TunnelPayload.Status":      classKeep,
	"TunnelPayload.Description": classScrub,
}

// identityKind splits hosts from usernames. Both are hidden unless opted in.
type identityKind string

const (
	identityHost identityKind = "host"
	identityUser identityKind = "user"
)

// A host field is an address. A user field is a username. Anything else is not
// an identity slot.
func identityOf(key string) identityKind {
	switch {
	case strings.HasSuffix(key, ".Host"), strings.HasSuffix(key, ".LocalHost"), strings.HasSuffix(key, ".RemoteHost"):
		return identityHost
	case strings.HasSuffix(key, ".User"):
		return identityUser
	default:
		return ""
	}
}

type scrubList struct {
	secrets []string
	hosts   []string
	users   []string
}

func (s scrubList) identities(includeHosts bool) []string {
	if includeHosts {
		return nil
	}
	out := make([]string, 0, len(s.hosts)+len(s.users))
	out = append(out, s.hosts...)
	out = append(out, s.users...)
	return out
}

// redactConfig copies cfg and replaces secret fields. Hosts and usernames are
// replaced unless includeHosts is set. extra secrets (the automation token)
// are never written; they are only used to cut matching text out of logs.
func redactConfig(cfg *conf.Config, includeHosts bool, extra []string) (*conf.Config, scrubList) {
	if cfg == nil {
		cfg = conf.DefaultConfig()
	}
	clone := cfg.Clone()
	list := collectScrub(clone, extra)
	redactSlice(reflect.ValueOf(&clone.Jumpers).Elem(), "Jumper", includeHosts, list)
	redactSlice(reflect.ValueOf(&clone.Tunnels).Elem(), "Tunnel", includeHosts, list)
	return clone, list
}

func collectScrub(cfg *conf.Config, extra []string) scrubList {
	var list scrubList
	gather := func(slice reflect.Value, structName string) {
		eachString(slice, func(field reflect.StructField, value reflect.Value) {
			text := strings.TrimSpace(value.String())
			if text == "" || text == Redacted {
				return
			}
			key := structName + "." + field.Name
			switch fieldClass[key] {
			case classSecret:
				list.secrets = append(list.secrets, text)
			case classIdentity:
				if identityOf(key) == identityUser {
					list.users = append(list.users, text)
					return
				}
				list.hosts = append(list.hosts, text)
			}
		})
	}
	gather(reflect.ValueOf(cfg.Jumpers), "Jumper")
	gather(reflect.ValueOf(cfg.Tunnels), "Tunnel")
	list.secrets = append(list.secrets, extra...)
	list.secrets = normalizeScrub(list.secrets)
	list.hosts = normalizeScrub(list.hosts)
	list.users = normalizeScrub(list.users)
	return list
}

func redactSlice(slice reflect.Value, structName string, includeHosts bool, list scrubList) {
	ids := list.identities(includeHosts)
	eachString(slice, func(field reflect.StructField, value reflect.Value) {
		if !value.CanSet() {
			return
		}
		current := value.String()
		if strings.TrimSpace(current) == "" {
			return
		}
		key := structName + "." + field.Name
		switch fieldClass[key] {
		case classSecret:
			value.SetString(Redacted)
		case classIdentity:
			if !includeHosts {
				value.SetString(Redacted)
				return
			}
			value.SetString(scrubText(current, list.secrets, nil))
		case classScrub:
			value.SetString(scrubText(current, list.secrets, ids))
		default:
			value.SetString(scrubText(current, list.secrets, ids))
		}
	})
}

func eachString(slice reflect.Value, fn func(reflect.StructField, reflect.Value)) {
	if slice.Kind() != reflect.Slice {
		return
	}
	for i := 0; i < slice.Len(); i++ {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Pointer {
			if elem.IsNil() {
				continue
			}
			elem = elem.Elem()
		}
		if elem.Kind() != reflect.Struct {
			continue
		}
		typ := elem.Type()
		for f := 0; f < typ.NumField(); f++ {
			field := typ.Field(f)
			if field.Type.Kind() != reflect.String {
				continue
			}
			fn(field, elem.Field(f))
		}
	}
}

func normalizeScrub(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < minScrubLen || value == Redacted {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// scrubText replaces known secrets and identities and cuts PEM private keys.
// It does not read key files. Callers pass values they already have in memory.
func scrubText(text string, secrets, identities []string) string {
	if text == "" {
		return text
	}
	for _, secret := range secrets {
		if len(secret) < minScrubLen {
			continue
		}
		text = strings.ReplaceAll(text, secret, Redacted)
	}
	for _, identity := range identities {
		if len(identity) < minScrubLen {
			continue
		}
		text = strings.ReplaceAll(text, identity, Redacted)
	}
	return privateKeyPattern{}.ReplaceAllString(text, Redacted)
}

// sshConfigTypes are the structs a new secret field would land on.
func sshConfigTypes() []reflect.Type {
	return []reflect.Type{
		reflect.TypeOf(model.Jumper{}),
		reflect.TypeOf(model.Tunnel{}),
		reflect.TypeOf(model.JumperPayload{}),
		reflect.TypeOf(model.TunnelPayload{}),
	}
}
