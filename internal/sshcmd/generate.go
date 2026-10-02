package sshcmd

import (
	"fmt"
	"strconv"
	"strings"

	"norka/internal/model"
)

// FormatTunnel prints an equivalent `ssh -N` command for one tunnel.
// Password authentication cannot be expressed, so a password is omitted.
func FormatTunnel(tunnel model.Tunnel, jumpers []model.Jumper) (string, error) {
	if len(jumpers) == 0 {
		return "", fmt.Errorf("tunnel has no jump host")
	}
	dest := jumpers[len(jumpers)-1]
	var args []string
	args = append(args, "ssh", "-N")
	forward, err := formatForward(tunnel)
	if err != nil {
		return "", err
	}
	args = append(args, forward...)
	if dest.Port > 0 && dest.Port != defaultPort {
		args = append(args, "-p", strconv.Itoa(dest.Port))
	}
	if dest.AuthType == "ssh_key" && strings.TrimSpace(dest.KeyPath) != "" {
		args = append(args, "-i", quoteArg(dest.KeyPath))
	}
	if ms := dest.KeepAliveIntervalMs; ms != defaultKeepAliveMs {
		args = append(args, "-o", "ServerAliveInterval="+strconv.Itoa(ms/1000))
	}
	if ms := dest.TimeoutMs; ms > 0 && ms != defaultTimeoutMs {
		args = append(args, "-o", "ConnectTimeout="+strconv.Itoa(ms/1000))
	}
	if len(jumpers) > 1 {
		hops := make([]string, 0, len(jumpers)-1)
		for _, hop := range jumpers[:len(jumpers)-1] {
			hops = append(hops, formatDestination(hop.User, hop.Host, hop.Port, true))
		}
		args = append(args, "-J", quoteArg(strings.Join(hops, ",")))
	}
	args = append(args, quoteArg(formatDestination(dest.User, dest.Host, dest.Port, false)))
	return strings.Join(args, " "), nil
}

func formatForward(tunnel model.Tunnel) ([]string, error) {
	mode := strings.TrimSpace(tunnel.Mode)
	if mode == "" {
		mode = "local"
	}
	switch mode {
	case "local":
		return []string{"-L", formatLocal(tunnel)}, nil
	case "remote":
		return []string{"-R", formatRemote(tunnel)}, nil
	case "dynamic":
		return []string{"-D", formatDynamic(tunnel)}, nil
	default:
		return nil, fmt.Errorf("unsupported tunnel mode %q", tunnel.Mode)
	}
}

func formatLocal(tunnel model.Tunnel) string {
	target := formatEndpoint(tunnel.RemoteHost, tunnel.RemotePort)
	if isLoopback(tunnel.LocalHost) {
		return strconv.Itoa(tunnel.LocalPort) + ":" + target
	}
	return formatEndpoint(tunnel.LocalHost, tunnel.LocalPort) + ":" + target
}

func formatRemote(tunnel model.Tunnel) string {
	target := formatEndpoint(tunnel.LocalHost, tunnel.LocalPort)
	if isLoopback(tunnel.RemoteHost) {
		return strconv.Itoa(tunnel.RemotePort) + ":" + target
	}
	return formatEndpoint(tunnel.RemoteHost, tunnel.RemotePort) + ":" + target
}

func formatDynamic(tunnel model.Tunnel) string {
	if isLoopback(tunnel.LocalHost) {
		return strconv.Itoa(tunnel.LocalPort)
	}
	return formatEndpoint(tunnel.LocalHost, tunnel.LocalPort)
}

func formatEndpoint(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(port)
}

func formatDestination(user, host string, port int, withPort bool) string {
	host = strings.TrimSpace(host)
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	dest := host
	if strings.TrimSpace(user) != "" {
		dest = strings.TrimSpace(user) + "@" + host
	}
	if withPort && port > 0 && port != defaultPort {
		dest += ":" + strconv.Itoa(port)
	}
	return dest
}

func isLoopback(host string) bool {
	switch strings.TrimSpace(host) {
	case "", "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func quoteArg(value string) string {
	if value == "" {
		return `""`
	}
	if !needsQuote(value) {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func needsQuote(value string) bool {
	return strings.ContainsAny(value, " \t\n\"'\\$`!*?[]{}()#&|;<>")
}
