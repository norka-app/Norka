package sshcmd

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"norka/internal/model"
)

func buildPreview(commands []command, jumpers []model.Jumper, tunnels []model.Tunnel) model.SSHCommandPreview {
	preview := model.SSHCommandPreview{
		Hosts:    []model.SSHCommandHost{},
		Tunnels:  []model.SSHCommandTunnel{},
		Warnings: []model.SSHCommandWarning{},
	}
	existingByKey := map[string]model.Jumper{}
	for _, jumper := range jumpers {
		key := hostKey(jumper.User, jumper.Host, jumper.Port)
		if _, ok := existingByKey[key]; ok {
			continue
		}
		existingByKey[key] = jumper
	}
	usedNames := map[string]struct{}{}
	for _, jumper := range jumpers {
		rememberName(usedNames, jumper.Name)
	}
	for _, tunnel := range tunnels {
		rememberName(usedNames, tunnel.Name)
	}

	hostOrder := []string{}
	hosts := map[string]model.SSHCommandHost{}
	seenTunnel := map[string]struct{}{}
	existingTunnel := existingTunnelKeys(tunnels, jumpers)

	for _, cmd := range commands {
		for _, item := range cmd.Warnings {
			preview.Warnings = append(preview.Warnings, item.model())
		}
		keys := make([]string, 0, len(cmd.Hosts))
		labels := make([]string, 0, len(cmd.Hosts))
		missingUser := false
		for _, item := range cmd.Hosts {
			key := hostKey(item.User, item.Host, item.Port)
			keys = append(keys, key)
			labels = append(labels, chainPart(item))
			if strings.TrimSpace(item.User) == "" || strings.TrimSpace(item.Host) == "" {
				missingUser = true
			}
			if _, ok := hosts[key]; ok {
				continue
			}
			row := model.SSHCommandHost{
				Key:                    key,
				Host:                   item.Host,
				Port:                   portOrDefault(item.Port),
				User:                   item.User,
				AuthType:               item.AuthType,
				KeyPath:                item.KeyPath,
				AgentSocketPath:        item.AgentSocketPath,
				KeepAliveIntervalMs:    item.KeepAliveIntervalMs,
				TimeoutMs:              item.TimeoutMs,
				BypassHostVerification: item.BypassHostVerification,
				HostKeyAlgorithms:      item.HostKeyAlgorithms,
				Alias:                  item.Alias,
				Ready:                  strings.TrimSpace(item.User) != "" && strings.TrimSpace(item.Host) != "",
			}
			if existing, ok := existingByKey[key]; ok {
				row.ExistingID = existing.ID
				row.ExistingName = existing.Name
				row.Name = existing.Name
				row.Ready = true
			} else {
				row.Name = fitName(suggestHostName(nameSource(item)), usedNames)
			}
			hosts[key] = row
			hostOrder = append(hostOrder, key)
		}
		chain := strings.Join(labels, " → ")
		for _, fwd := range cmd.Forwards {
			tunnel := model.SSHCommandTunnel{
				Mode:       fwd.Mode,
				LocalHost:  fwd.LocalHost,
				LocalPort:  fwd.LocalPort,
				RemoteHost: fwd.RemoteHost,
				RemotePort: fwd.RemotePort,
				HostKeys:   append([]string{}, keys...),
				ChainLabel: chain,
			}
			signature := tunnelSignature(tunnel)
			if missingUser || len(keys) == 0 {
				tunnel.Blocked = true
				tunnel.BlockReason = WarnMissingUser
			} else if _, seen := seenTunnel[signature]; seen {
				tunnel.Blocked = true
				tunnel.BlockReason = WarnDuplicateBatch
				preview.Warnings = append(preview.Warnings, model.SSHCommandWarning{Code: WarnDuplicateBatch})
			} else {
				seenTunnel[signature] = struct{}{}
				if _, exists := existingTunnel[signature]; exists {
					tunnel.Blocked = true
					tunnel.BlockReason = WarnDuplicateExisting
					preview.Warnings = append(preview.Warnings, model.SSHCommandWarning{Code: WarnDuplicateExisting})
				}
			}
			tunnel.Name = fitName(suggestTunnelName(fwd, cmd.Hosts), usedNames)
			preview.Tunnels = append(preview.Tunnels, tunnel)
		}
	}
	for _, key := range hostOrder {
		preview.Hosts = append(preview.Hosts, hosts[key])
	}
	return preview
}

func existingTunnelKeys(tunnels []model.Tunnel, jumpers []model.Jumper) map[string]struct{} {
	byID := map[int]model.Jumper{}
	for _, jumper := range jumpers {
		byID[jumper.ID] = jumper
	}
	out := map[string]struct{}{}
	for _, tunnel := range tunnels {
		keys := make([]string, 0, len(tunnel.JumperIDs))
		for _, id := range tunnel.JumperIDs {
			jumper, ok := byID[id]
			if !ok {
				keys = nil
				break
			}
			keys = append(keys, hostKey(jumper.User, jumper.Host, jumper.Port))
		}
		if len(keys) == 0 {
			continue
		}
		item := model.SSHCommandTunnel{
			Mode:       tunnel.Mode,
			LocalHost:  tunnel.LocalHost,
			LocalPort:  tunnel.LocalPort,
			RemoteHost: tunnel.RemoteHost,
			RemotePort: tunnel.RemotePort,
			HostKeys:   keys,
		}
		out[tunnelSignature(item)] = struct{}{}
	}
	return out
}

func tunnelSignature(tunnel model.SSHCommandTunnel) string {
	mode := strings.ToLower(strings.TrimSpace(tunnel.Mode))
	localHost := strings.ToLower(strings.TrimSpace(tunnel.LocalHost))
	remoteHost := ""
	remotePort := 0
	if mode != "dynamic" {
		remoteHost = strings.ToLower(strings.TrimSpace(tunnel.RemoteHost))
		remotePort = tunnel.RemotePort
	}
	return mode + "|" + localHost + "|" + strconv.Itoa(tunnel.LocalPort) + "|" + remoteHost + "|" + strconv.Itoa(remotePort) + "|" + strings.Join(tunnel.HostKeys, ">")
}

func hostKey(user, host string, port int) string {
	return strings.TrimSpace(user) + "@" + strings.ToLower(strings.TrimSpace(host)) + ":" + strconv.Itoa(portOrDefault(port))
}

func portOrDefault(port int) int {
	if port <= 0 {
		return defaultPort
	}
	return port
}

func chainPart(item host) string {
	port := portOrDefault(item.Port)
	if strings.TrimSpace(item.User) == "" {
		return item.Host + ":" + strconv.Itoa(port)
	}
	return item.User + "@" + item.Host + ":" + strconv.Itoa(port)
}

func suggestHostName(host string) string {
	name := strings.TrimSpace(host)
	if idx := strings.Index(name, "."); idx > 0 {
		name = name[:idx]
	}
	return name
}

func nameSource(item host) string {
	if strings.TrimSpace(item.Alias) != "" {
		return item.Alias
	}
	return item.Host
}

func suggestTunnelName(fwd forward, hosts []host) string {
	label := "tun"
	if len(hosts) > 0 {
		label = suggestHostName(nameSource(hosts[len(hosts)-1]))
	}
	port := fwd.LocalPort
	if fwd.Mode != "dynamic" && fwd.Mode != "remote" {
		port = fwd.RemotePort
	}
	if fwd.Mode == "remote" {
		port = fwd.RemotePort
	}
	if label == "" {
		label = fwd.Mode
	}
	return label + "-" + strconv.Itoa(port)
}

func fitName(base string, used map[string]struct{}) string {
	base = sanitizeName(base)
	if base == "" {
		base = "host"
	}
	base = trimUnits(base, 20)
	if rememberName(used, base) {
		return base
	}
	for n := 2; n < 100; n++ {
		suffix := "-" + strconv.Itoa(n)
		head := trimUnits(base, 20-len(suffix))
		if head == "" {
			head = "h"
		}
		candidate := head + suffix
		if rememberName(used, candidate) {
			return candidate
		}
	}
	return base
}

func rememberName(used map[string]struct{}, name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return false
	}
	if _, ok := used[key]; ok {
		return false
	}
	used[key] = struct{}{}
	return true
}

func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

func trimUnits(name string, max int) string {
	if max <= 0 {
		return ""
	}
	units := 0
	end := 0
	for i, r := range name {
		weight := 1
		if r >= 0x3400 && r <= 0x9fff {
			weight = 2
		}
		if units+weight > max {
			break
		}
		units += weight
		end = i + utf8.RuneLen(r)
	}
	return name[:end]
}
