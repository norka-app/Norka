package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/norka-app/Norka/internal/ipc"
	"github.com/norka-app/Norka/internal/localbind"
)

func usageText() string {
	return strings.TrimSpace(`
norka-cli controls tunnels in a running Norka app.

Usage:
  norka-cli connect <name|id>      connect a tunnel
  norka-cli disconnect <name|id>   disconnect a tunnel
  norka-cli toggle <name|id>       toggle a tunnel
  norka-cli status [--json]        Norka and tunnel status
  norka-cli list [--json]          list tunnels
  norka-cli daemon stop [--force]  ask the owner to stop and exit
  norka-cli daemon handover        ask the owner to drop the engine lock
  norka-cli --version              print the client version

Norka must be running, and Automation must be on in Settings → Features.
If Norka is not running, connect starts it in the tray. The app records its
executable next to config.toml when it starts. disconnect, toggle, status,
list, and daemon do not start the app.

status --json speaks protocol 2: it prints hello (owner, pid, version) and
the full state. daemon stop asks the owner to stop its tunnels and exit.
A window ignores that unless --force is set. daemon handover asks the owner
to stop its tunnels and release the engine lock.

Exit codes:
  0  success
  1  automation is off (Settings → Features)
  2  Norka is not running
  3  tunnel was not found
  4  the name is ambiguous
  5  invalid arguments
  6  the local channel could not be reached
  7  the tunnel command failed
  8  Norka and norka-cli speak different versions
`)
}

func writeHuman(w io.Writer, text string) {
	if w == nil || strings.TrimSpace(text) == "" {
		return
	}
	_, _ = io.WriteString(w, text)
	if !strings.HasSuffix(text, "\n") {
		_, _ = io.WriteString(w, "\n")
	}
}

func writeJSON(w io.Writer, resp ipc.Response) error {
	if w == nil {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		return err
	}
	_, err := io.WriteString(w, strings.TrimRight(buf.String(), "\n")+"\n")
	return err
}

func explain(cmd command, resp ipc.Response) string {
	switch resp.Code {
	case ipc.CodeDisabled:
		return "Automation is off. Turn it on in Settings → Features."
	case ipc.CodeNotRunning:
		if strings.TrimSpace(resp.Message) != "" {
			return resp.Message
		}
		return "Norka is not running."
	case ipc.CodeNotFound:
		return fmt.Sprintf("Tunnel %q was not found.", cmd.Target)
	case ipc.CodeAmbiguous:
		return fmt.Sprintf("%q matches more than one tunnel: %s. Use the full name or the id.", cmd.Target, strings.Join(resp.Names, ", "))
	case ipc.CodeUpdateApp:
		return ipc.MessageUpdateApp
	case ipc.CodeUpdateCLI:
		return ipc.MessageUpdateCLI
	case ipc.CodeIgnored:
		return "The window ignored shutdown. Pass --force to stop it."
	case ipc.CodeTimeout:
		if strings.TrimSpace(resp.Message) != "" {
			return resp.Message
		}
		return "Handover timed out. The owner still holds the engine lock."
	case ipc.CodeUnauthorized:
		return "Access to the local Norka channel was denied."
	case ipc.CodeOK:
		return okText(cmd, resp)
	case ipc.CodeFailed:
		return failedText(cmd, resp)
	default:
		if strings.TrimSpace(resp.Message) != "" {
			return resp.Message
		}
		if resp.Code != "" {
			return resp.Code
		}
		return "The command failed."
	}
}

func okText(cmd command, resp ipc.Response) string {
	if cmd.Op == ipc.OpShutdown {
		return "Norka is stopping."
	}
	if cmd.Op == ipc.OpHandover {
		return "The engine lock is released."
	}
	if cmd.Op == ipc.OpStatus || cmd.Op == ipc.OpList {
		return statusLine(resp.Tunnels)
	}
	if len(resp.Tunnels) == 0 {
		return "OK"
	}
	name := resp.Tunnels[0].Name
	if name == "" {
		name = cmd.Target
	}
	if cmd.Op == ipc.OpDisconnect || resp.Tunnels[0].Status == "stopped" {
		return fmt.Sprintf("Tunnel %q is disconnected.", name)
	}
	return fmt.Sprintf("Tunnel %q is connected.", name)
}

func failedText(cmd command, resp ipc.Response) string {
	name := cmd.Target
	detail := strings.TrimSpace(resp.Message)
	if len(resp.Tunnels) > 0 {
		if resp.Tunnels[0].Name != "" {
			name = resp.Tunnels[0].Name
		}
		if strings.TrimSpace(resp.Tunnels[0].LastError) != "" {
			detail = resp.Tunnels[0].LastError
		}
	}
	if detail == "" {
		detail = "unknown error"
	}
	detail = localbind.Present(detail)
	if name == "" {
		return "The command failed: " + detail
	}
	return fmt.Sprintf("The command failed for %q: %s", name, detail)
}

func statusLine(tunnels []ipc.TunnelInfo) string {
	if len(tunnels) == 0 {
		return "Norka is running. There are no tunnels."
	}
	running := 0
	for _, tunnel := range tunnels {
		if tunnel.Status == "running" || tunnel.Status == "reconnecting" {
			running++
		}
	}
	return fmt.Sprintf("Norka is running. Tunnels: %d, connected: %d.", len(tunnels), running)
}

func tunnelTable(tunnels []ipc.TunnelInfo) string {
	if len(tunnels) == 0 {
		return ""
	}
	var buf strings.Builder
	for _, tunnel := range tunnels {
		fmt.Fprintf(&buf, "%d\t%s\t%s\t%s\t%s\n", tunnel.ID, tunnel.Name, tunnel.Status, tunnel.Mode, addressOf(tunnel))
	}
	return buf.String()
}

func addressOf(tunnel ipc.TunnelInfo) string {
	host := tunnel.LocalHost
	if host == "" {
		host = "127.0.0.1"
	}
	if tunnel.LocalPort <= 0 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, tunnel.LocalPort)
}
