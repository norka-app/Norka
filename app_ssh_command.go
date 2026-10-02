package main

import (
	"fmt"

	"norka/internal/features"
	"norka/internal/model"
	"norka/internal/sshcmd"
	"norka/internal/sshconfig"
)

// PreviewSSHCommands parses pasted ssh lines into tunnels and jump hosts.
// Existing jump hosts are reused when user, host and port match.
// Nothing secret from the command line is returned.
func (a *App) PreviewSSHCommands(text string) (model.SSHCommandPreview, error) {
	if err := a.ensureReady(); err != nil {
		return model.SSHCommandPreview{}, err
	}
	if !a.featureOn(features.SSHCommand) {
		return model.SSHCommandPreview{}, fmt.Errorf("ssh command is disabled")
	}
	jumpers, err := a.jumper.List()
	if err != nil {
		return model.SSHCommandPreview{}, err
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return model.SSHCommandPreview{}, err
	}
	return sshcmd.Preview(text, sshAliasResolver(), jumpers, tunnels)
}

// FormatTunnelSSHCommand copies one tunnel as an ssh -N command.
// Password authentication is omitted because ssh cannot express it.
func (a *App) FormatTunnelSSHCommand(tunnelID int) (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}
	if !a.featureOn(features.SSHCommand) {
		return "", fmt.Errorf("ssh command is disabled")
	}
	tunnels, err := a.tunnel.List()
	if err != nil {
		return "", err
	}
	var tunnel model.Tunnel
	found := false
	for _, item := range tunnels {
		if item.ID == tunnelID {
			tunnel = item
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("tunnel not found")
	}
	jumpers, err := a.jumper.List()
	if err != nil {
		return "", err
	}
	byID := make(map[int]model.Jumper, len(jumpers))
	for _, jumper := range jumpers {
		byID[jumper.ID] = jumper
	}
	chain := make([]model.Jumper, 0, len(tunnel.JumperIDs))
	for _, id := range tunnel.JumperIDs {
		jumper, ok := byID[id]
		if !ok {
			return "", fmt.Errorf("jump host not found")
		}
		chain = append(chain, jumper)
	}
	return sshcmd.FormatTunnel(tunnel, chain)
}

func sshAliasResolver() sshcmd.Resolver {
	return func(configFile, name string) (sshcmd.Alias, bool, error) {
		candidate, ok, err := sshconfig.LookupAlias(configFile, name)
		if err != nil || !ok {
			return sshcmd.Alias{}, false, err
		}
		return sshcmd.Alias{
			Name:                   candidate.Alias,
			Host:                   candidate.Host,
			Port:                   candidate.Port,
			User:                   candidate.User,
			KeyPath:                candidate.KeyPath,
			AgentSocketPath:        candidate.AgentSocketPath,
			KeepAliveIntervalMs:    candidate.KeepAliveIntervalMs,
			TimeoutMs:              candidate.TimeoutMs,
			BypassHostVerification: candidate.BypassHostVerification,
			HostKeyAlgorithms:      candidate.HostKeyAlgorithms,
			ProxyJump:              candidate.ProxyJump,
		}, true, nil
	}
}
