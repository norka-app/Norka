// Package sshcmd parses and prints ssh port-forward commands.
//
// The parser tokenizes POSIX and Windows cmd quoting, turns each -L, -R and -D
// into its own forward, and can resolve Host aliases through a caller-supplied
// lookup (the app uses the ~/.ssh/config importer). Secrets from the command
// line are never copied into the result.
package sshcmd

import (
	"strconv"
	"strings"

	"github.com/norka-app/Norka/internal/model"
)

const (
	defaultPort        = 22
	defaultKeepAliveMs = 5000
	defaultTimeoutMs   = 10000
	maxKeepAliveMs     = 120000
	maxTimeoutMs       = 120000
	minTimeoutMs       = 100
	minKeepAliveMs     = 1000
)

// Alias is one explicit Host block from an SSH config file.
type Alias struct {
	Name                   string
	Host                   string
	Port                   int
	User                   string
	KeyPath                string
	AgentSocketPath        string
	KeepAliveIntervalMs    int
	TimeoutMs              int
	BypassHostVerification bool
	HostKeyAlgorithms      string
	ProxyJump              string
}

// Resolver looks up an explicit Host alias. configFile is the -F path, or empty
// for the default ~/.ssh/config. ok is false when the name is not an alias.
type Resolver func(configFile, name string) (Alias, bool, error)

// Warning codes returned by Preview. Option and Detail are safe to show.
const (
	WarnUnknownOption     = "unknown_option"
	WarnInvalidForward    = "invalid_forward"
	WarnMissingTarget     = "missing_target"
	WarnMissingUser       = "missing_user"
	WarnIgnoredSecret     = "ignored_secret"
	WarnExtraIdentity     = "extra_identity"
	WarnNotSSH            = "not_ssh"
	WarnNoForward         = "no_forward"
	WarnSSHConfig         = "ssh_config"
	WarnDuplicateExisting = "duplicate_existing"
	WarnDuplicateBatch    = "duplicate_batch"
)

type warning struct {
	Code   string
	Option string
	Detail string
}

func (w warning) model() model.SSHCommandWarning {
	return model.SSHCommandWarning{Code: w.Code, Option: w.Option, Detail: w.Detail}
}

type forward struct {
	Mode       string
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
}

type host struct {
	Alias                  string
	Host                   string
	Port                   int
	User                   string
	KeyPath                string
	AgentSocketPath        string
	KeepAliveIntervalMs    int
	TimeoutMs              int
	BypassHostVerification bool
	HostKeyAlgorithms      string
	AuthType               string
}

type command struct {
	Hosts    []host
	Forwards []forward
	Warnings []warning
}

type rawEndpoint struct {
	User string
	Host string
	Port int // 0 means the token did not set a port
}

type parsedCLI struct {
	configFile   string
	user         string
	userSet      bool
	port         int
	portSet      bool
	keyPath      string
	keySet       bool
	jumps        []string
	jumpsSet     bool
	keepAliveSec *int
	timeoutSec   *int
	agentSocket  string
	agentSet     bool
	bypass       *bool
	algorithms   string
	forwards     []forward
	warnings     []warning
}

var optionTakesArg = map[byte]bool{
	'B': true, 'b': true, 'c': true, 'D': true, 'E': true, 'e': true,
	'F': true, 'I': true, 'i': true, 'J': true, 'L': true, 'l': true,
	'm': true, 'O': true, 'o': true, 'p': true, 'Q': true, 'R': true,
	'S': true, 'W': true, 'w': true,
}

var harmlessFlag = map[byte]bool{
	'f': true, 'N': true, 'T': true, 'C': true, 'q': true, 'v': true,
}

// Preview parses one or more ssh command lines and matches jump hosts that
// already exist. Passwords and other secrets are dropped before the result is built.
func Preview(text string, resolve Resolver, jumpers []model.Jumper, tunnels []model.Tunnel) (model.SSHCommandPreview, error) {
	commands := parseAll(text, resolve)
	return buildPreview(commands, jumpers, tunnels), nil
}

func parseAll(text string, resolve Resolver) []command {
	var commands []command
	for _, line := range splitCommands(text) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		tokens, err := tokenizeLine(trimmed)
		if err != nil || len(tokens) == 0 {
			continue
		}
		commands = append(commands, parseTokens(tokens, resolve))
	}
	return commands
}

func parseTokens(tokens []string, resolve Resolver) command {
	tokens, secretWarnings := stripSSHPass(tokens)
	if len(tokens) == 0 {
		return command{Warnings: secretWarnings}
	}
	if !isSSHBinary(tokens[0]) && !strings.HasPrefix(tokens[0], "-") {
		return command{Warnings: append(secretWarnings, warning{Code: WarnNotSSH, Detail: safeToken(tokens[0])})}
	}
	if isSSHBinary(tokens[0]) {
		tokens = tokens[1:]
	}

	cli, dest, okDest := parseArgs(tokens)
	cli.warnings = append(secretWarnings, cli.warnings...)
	if !okDest || strings.TrimSpace(dest.Host) == "" {
		cli.warnings = append(cli.warnings, warning{Code: WarnMissingTarget})
		return command{Warnings: cli.warnings}
	}
	if len(cli.forwards) == 0 {
		cli.warnings = append(cli.warnings, warning{Code: WarnNoForward})
	}

	destination, jumps, warnings := resolveDestination(dest, cli, resolve)
	warnings = append(cli.warnings, warnings...)
	if strings.TrimSpace(destination.User) == "" {
		warnings = append(warnings, warning{Code: WarnMissingUser, Detail: destination.Host})
	}
	hosts := append(jumps, destination)
	return command{Hosts: hosts, Forwards: cli.forwards, Warnings: warnings}
}

func parseArgs(tokens []string) (parsedCLI, rawEndpoint, bool) {
	var cli parsedCLI
	var dest rawEndpoint
	hasDest := false
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok == "--" {
			if i+1 < len(tokens) {
				dest = parseEndpoint(tokens[i+1], true)
				hasDest = dest.Host != ""
			}
			break
		}
		if strings.HasPrefix(tok, "-") && tok != "-" && !strings.HasPrefix(tok, "---") {
			next, consumed := applyOption(tok, tokens, i, &cli)
			i = next
			if consumed {
				continue
			}
		}
		dest = parseEndpoint(tok, true)
		hasDest = dest.Host != ""
		break
	}
	return cli, dest, hasDest
}

// applyOption consumes tok at index i. The returned index is the last consumed token.
func applyOption(tok string, tokens []string, i int, cli *parsedCLI) (int, bool) {
	if strings.HasPrefix(tok, "--") {
		cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: safeToken(tok)})
		return i, true
	}
	body := tok[1:]
	for n := 0; n < len(body); n++ {
		flag := body[n]
		rest := body[n+1:]
		if optionTakesArg[flag] {
			arg := rest
			if arg == "" {
				if i+1 >= len(tokens) {
					cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-" + string(flag)})
					return i, true
				}
				i++
				arg = tokens[i]
			}
			applyArg(flag, arg, cli)
			return i, true
		}
		if harmlessFlag[flag] {
			continue
		}
		cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-" + string(flag)})
	}
	return i, true
}

func applyArg(flag byte, arg string, cli *parsedCLI) {
	switch flag {
	case 'L':
		if fwd, ok := parseLocal(arg); ok {
			cli.forwards = append(cli.forwards, fwd)
		} else {
			cli.warnings = append(cli.warnings, warning{Code: WarnInvalidForward, Option: "-L", Detail: arg})
		}
	case 'R':
		if fwd, ok := parseRemote(arg); ok {
			cli.forwards = append(cli.forwards, fwd)
		} else {
			cli.warnings = append(cli.warnings, warning{Code: WarnInvalidForward, Option: "-R", Detail: arg})
		}
	case 'D':
		if fwd, ok := parseDynamic(arg); ok {
			cli.forwards = append(cli.forwards, fwd)
		} else {
			cli.warnings = append(cli.warnings, warning{Code: WarnInvalidForward, Option: "-D", Detail: arg})
		}
	case 'p':
		port, err := strconv.Atoi(arg)
		if err != nil || port < 1 || port > 65535 {
			cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-p"})
			return
		}
		cli.port = port
		cli.portSet = true
	case 'l':
		cli.user = arg
		cli.userSet = true
	case 'i':
		if cli.keySet {
			cli.warnings = append(cli.warnings, warning{Code: WarnExtraIdentity, Option: "-i"})
			return
		}
		cli.keyPath = arg
		cli.keySet = true
	case 'J':
		if strings.EqualFold(strings.TrimSpace(arg), "none") {
			cli.jumps = nil
			cli.jumpsSet = true
			return
		}
		cli.jumps = append(cli.jumps, splitJumpList(arg)...)
		cli.jumpsSet = true
	case 'F':
		cli.configFile = arg
	case 'o':
		applyConfigOption(arg, cli)
	default:
		cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-" + string(flag)})
	}
}

func applyConfigOption(arg string, cli *parsedCLI) {
	key, value, ok := splitOption(arg)
	if !ok {
		cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-o"})
		return
	}
	if isSecretOption(key) {
		cli.warnings = append(cli.warnings, warning{Code: WarnIgnoredSecret, Option: "-o " + key})
		return
	}
	switch strings.ToLower(key) {
	case "serveraliveinterval":
		sec, valid := parseNonNegInt(value)
		if !valid {
			cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-o ServerAliveInterval"})
			return
		}
		cli.keepAliveSec = &sec
	case "connecttimeout":
		sec, valid := parsePositiveInt(value)
		if !valid {
			cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-o ConnectTimeout"})
			return
		}
		cli.timeoutSec = &sec
	case "proxyjump":
		if strings.EqualFold(strings.TrimSpace(value), "none") {
			cli.jumps = nil
			cli.jumpsSet = true
			return
		}
		cli.jumps = append(cli.jumps, splitJumpList(value)...)
		cli.jumpsSet = true
	case "user":
		cli.user = value
		cli.userSet = true
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-o Port"})
			return
		}
		cli.port = port
		cli.portSet = true
	case "identityfile":
		if cli.keySet {
			cli.warnings = append(cli.warnings, warning{Code: WarnExtraIdentity, Option: "-o IdentityFile"})
			return
		}
		cli.keyPath = value
		cli.keySet = true
	case "identityagent":
		cli.agentSet = true
		if strings.EqualFold(value, "none") {
			cli.agentSocket = ""
			return
		}
		cli.agentSocket = value
	case "stricthostkeychecking":
		switch strings.ToLower(value) {
		case "no", "off":
			on := true
			cli.bypass = &on
		case "yes", "ask", "accept-new":
			off := false
			cli.bypass = &off
		default:
			cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-o StrictHostKeyChecking"})
		}
	case "hostkeyalgorithms":
		cli.algorithms = value
	default:
		cli.warnings = append(cli.warnings, warning{Code: WarnUnknownOption, Option: "-o " + key})
	}
}

func isSecretOption(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch lower {
	case "proxycommand", "localcommand", "remotecommand", "knownhostscommand":
		return true
	}
	return strings.Contains(lower, "pass") || strings.Contains(lower, "secret") || strings.Contains(lower, "token")
}

func splitOption(arg string) (string, string, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", false
	}
	if idx := strings.IndexAny(arg, "= \t"); idx >= 0 {
		key := strings.TrimSpace(arg[:idx])
		value := strings.TrimSpace(arg[idx+1:])
		if key == "" {
			return "", "", false
		}
		return key, value, true
	}
	return arg, "", false
}

func resolveDestination(dest rawEndpoint, cli parsedCLI, resolve Resolver) (host, []host, []warning) {
	var warnings []warning
	base := host{
		Host:                dest.Host,
		Port:                defaultPort,
		User:                dest.User,
		KeepAliveIntervalMs: defaultKeepAliveMs,
		TimeoutMs:           defaultTimeoutMs,
		AuthType:            "ssh_agent",
	}
	if dest.Port > 0 {
		base.Port = dest.Port
	}
	var proxyFromAlias string
	if resolve != nil && dest.Host != "" {
		alias, ok, err := resolve(cli.configFile, dest.Host)
		if err != nil {
			warnings = append(warnings, warning{Code: WarnSSHConfig, Detail: err.Error()})
		} else if ok {
			base.Alias = alias.Name
			if strings.TrimSpace(alias.Host) != "" {
				base.Host = alias.Host
			}
			if base.User == "" {
				base.User = alias.User
			}
			if dest.Port == 0 && alias.Port > 0 {
				base.Port = alias.Port
			}
			if alias.KeyPath != "" {
				base.KeyPath = alias.KeyPath
			}
			if alias.AgentSocketPath != "" {
				base.AgentSocketPath = alias.AgentSocketPath
			}
			if alias.KeepAliveIntervalMs > 0 {
				base.KeepAliveIntervalMs = normalizeKeepAlive(alias.KeepAliveIntervalMs)
			}
			if alias.TimeoutMs > 0 {
				base.TimeoutMs = normalizeTimeout(alias.TimeoutMs)
			}
			base.BypassHostVerification = alias.BypassHostVerification
			base.HostKeyAlgorithms = alias.HostKeyAlgorithms
			proxyFromAlias = alias.ProxyJump
		}
	}
	if cli.userSet {
		base.User = cli.user
	}
	if cli.portSet {
		base.Port = cli.port
	}
	if cli.keySet {
		base.KeyPath = cli.keyPath
	}
	if cli.keepAliveSec != nil {
		base.KeepAliveIntervalMs = normalizeKeepAlive(*cli.keepAliveSec * 1000)
	}
	if cli.timeoutSec != nil {
		base.TimeoutMs = normalizeTimeout(*cli.timeoutSec * 1000)
	}
	if cli.agentSet {
		base.AgentSocketPath = cli.agentSocket
	}
	if cli.bypass != nil {
		base.BypassHostVerification = *cli.bypass
	}
	if cli.algorithms != "" {
		base.HostKeyAlgorithms = cli.algorithms
	}
	base.AuthType = authType(base)
	if base.AuthType != "ssh_key" {
		base.KeyPath = ""
	}
	if base.AuthType != "ssh_agent" {
		base.AgentSocketPath = ""
	}

	jumpSpecs := cli.jumps
	if !cli.jumpsSet {
		jumpSpecs = splitJumpList(proxyFromAlias)
	}
	jumps, jumpWarnings := resolveJumps(jumpSpecs, cli.configFile, resolve)
	return base, jumps, append(warnings, jumpWarnings...)
}

func resolveJumps(specs []string, configFile string, resolve Resolver) ([]host, []warning) {
	var hosts []host
	var warnings []warning
	seen := map[string]struct{}{}
	var walk func(spec string)
	walk = func(spec string) {
		spec = strings.TrimSpace(spec)
		if spec == "" || strings.EqualFold(spec, "none") {
			return
		}
		ep := parseEndpoint(spec, true)
		if ep.Host == "" {
			warnings = append(warnings, warning{Code: WarnInvalidForward, Option: "-J", Detail: spec})
			return
		}
		key := strings.ToLower(ep.Host)
		if _, ok := seen[key]; ok {
			warnings = append(warnings, warning{Code: WarnSSHConfig, Detail: ep.Host})
			return
		}
		item := host{
			Host:                ep.Host,
			Port:                defaultPort,
			User:                ep.User,
			KeepAliveIntervalMs: defaultKeepAliveMs,
			TimeoutMs:           defaultTimeoutMs,
			AuthType:            "ssh_agent",
		}
		if ep.Port > 0 {
			item.Port = ep.Port
		}
		var nested string
		if resolve != nil {
			seen[key] = struct{}{}
			alias, ok, err := resolve(configFile, ep.Host)
			delete(seen, key)
			if err != nil {
				warnings = append(warnings, warning{Code: WarnSSHConfig, Detail: err.Error()})
			} else if ok {
				seen[key] = struct{}{}
				item.Alias = alias.Name
				if strings.TrimSpace(alias.Host) != "" {
					item.Host = alias.Host
				}
				if item.User == "" {
					item.User = alias.User
				}
				if ep.Port == 0 && alias.Port > 0 {
					item.Port = alias.Port
				}
				if alias.KeyPath != "" {
					item.KeyPath = alias.KeyPath
					item.AuthType = "ssh_key"
				}
				if alias.AgentSocketPath != "" && item.AuthType != "ssh_key" {
					item.AgentSocketPath = alias.AgentSocketPath
				}
				if alias.KeepAliveIntervalMs > 0 {
					item.KeepAliveIntervalMs = normalizeKeepAlive(alias.KeepAliveIntervalMs)
				}
				if alias.TimeoutMs > 0 {
					item.TimeoutMs = normalizeTimeout(alias.TimeoutMs)
				}
				item.BypassHostVerification = alias.BypassHostVerification
				item.HostKeyAlgorithms = alias.HostKeyAlgorithms
				nested = alias.ProxyJump
			}
		}
		if item.User == "" {
			warnings = append(warnings, warning{Code: WarnMissingUser, Detail: item.Host})
		}
		item.AuthType = authType(item)
		if nested != "" {
			seen[key] = struct{}{}
			for _, hop := range splitJumpList(nested) {
				walk(hop)
			}
			delete(seen, key)
		}
		hosts = append(hosts, item)
	}
	for _, spec := range specs {
		walk(spec)
	}
	return hosts, warnings
}

func authType(item host) string {
	if strings.TrimSpace(item.KeyPath) != "" {
		return "ssh_key"
	}
	return "ssh_agent"
}

func normalizeKeepAlive(ms int) int {
	if ms <= 0 {
		return 0
	}
	if ms < minKeepAliveMs {
		return minKeepAliveMs
	}
	if ms > maxKeepAliveMs {
		return maxKeepAliveMs
	}
	return ms
}

func normalizeTimeout(ms int) int {
	if ms < minTimeoutMs {
		return minTimeoutMs
	}
	if ms > maxTimeoutMs {
		return maxTimeoutMs
	}
	return ms
}

func parseLocal(spec string) (forward, bool) {
	parts := splitSSHSpec(spec)
	fwd := forward{Mode: "local", LocalHost: "127.0.0.1"}
	switch len(parts) {
	case 3:
		fwd.LocalPort = atoiPort(parts[0])
		fwd.RemoteHost = parts[1]
		fwd.RemotePort = atoiPort(parts[2])
	case 4:
		fwd.LocalHost = defaultHost(parts[0])
		fwd.LocalPort = atoiPort(parts[1])
		fwd.RemoteHost = parts[2]
		fwd.RemotePort = atoiPort(parts[3])
	default:
		return forward{}, false
	}
	if !validForward(fwd) {
		return forward{}, false
	}
	return fwd, true
}

func parseRemote(spec string) (forward, bool) {
	parts := splitSSHSpec(spec)
	fwd := forward{Mode: "remote", RemoteHost: "127.0.0.1"}
	switch len(parts) {
	case 3:
		fwd.RemotePort = atoiPort(parts[0])
		fwd.LocalHost = parts[1]
		fwd.LocalPort = atoiPort(parts[2])
	case 4:
		fwd.RemoteHost = defaultHost(parts[0])
		fwd.RemotePort = atoiPort(parts[1])
		fwd.LocalHost = parts[2]
		fwd.LocalPort = atoiPort(parts[3])
	default:
		return forward{}, false
	}
	if !validForward(fwd) {
		return forward{}, false
	}
	return fwd, true
}

func parseDynamic(spec string) (forward, bool) {
	parts := splitSSHSpec(spec)
	fwd := forward{Mode: "dynamic", LocalHost: "127.0.0.1"}
	switch len(parts) {
	case 1:
		fwd.LocalPort = atoiPort(parts[0])
	case 2:
		fwd.LocalHost = defaultHost(parts[0])
		fwd.LocalPort = atoiPort(parts[1])
	default:
		return forward{}, false
	}
	if fwd.LocalHost == "" || !validPort(fwd.LocalPort) {
		return forward{}, false
	}
	return fwd, true
}

func validForward(fwd forward) bool {
	if strings.TrimSpace(fwd.LocalHost) == "" || strings.TrimSpace(fwd.RemoteHost) == "" {
		return false
	}
	return validPort(fwd.LocalPort) && validPort(fwd.RemotePort)
}

func validPort(port int) bool {
	return port >= 1 && port <= 65535
}

func atoiPort(raw string) int {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return port
}

func defaultHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" || host == "*" {
		return "127.0.0.1"
	}
	return host
}

func splitSSHSpec(spec string) []string {
	var parts []string
	var cur strings.Builder
	inBracket := false
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		switch {
		case c == '[' && !inBracket && cur.Len() == 0:
			inBracket = true
		case c == ']' && inBracket:
			inBracket = false
		case c == ':' && !inBracket:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	parts = append(parts, cur.String())
	return parts
}

func splitJumpList(spec string) []string {
	spec = strings.TrimSpace(spec)
	if spec == "" || strings.EqualFold(spec, "none") {
		return nil
	}
	var parts []string
	var cur strings.Builder
	inBracket := false
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		switch {
		case c == '[':
			inBracket = true
			cur.WriteByte(c)
		case c == ']':
			inBracket = false
			cur.WriteByte(c)
		case c == ',' && !inBracket:
			if item := strings.TrimSpace(cur.String()); item != "" {
				parts = append(parts, item)
			}
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if item := strings.TrimSpace(cur.String()); item != "" {
		parts = append(parts, item)
	}
	return parts
}

func parseEndpoint(token string, allowPort bool) rawEndpoint {
	token = strings.TrimSpace(token)
	if token == "" || strings.HasPrefix(token, "-") {
		return rawEndpoint{}
	}
	user := ""
	hostport := token
	if at := strings.LastIndex(token, "@"); at > 0 {
		user = token[:at]
		hostport = token[at+1:]
	}
	if user == "" && strings.Contains(token, "@") {
		return rawEndpoint{}
	}
	host, port := splitHostPort(hostport, allowPort)
	if host == "" {
		return rawEndpoint{}
	}
	return rawEndpoint{User: user, Host: host, Port: port}
}

func splitHostPort(hostport string, allowPort bool) (string, int) {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return "", 0
	}
	if strings.HasPrefix(hostport, "[") {
		end := strings.Index(hostport, "]")
		if end <= 1 {
			return "", 0
		}
		host := hostport[1:end]
		rest := hostport[end+1:]
		if rest == "" {
			return host, 0
		}
		if !allowPort || !strings.HasPrefix(rest, ":") {
			return "", 0
		}
		port, err := strconv.Atoi(rest[1:])
		if err != nil || !validPort(port) {
			return "", 0
		}
		return host, port
	}
	if allowPort {
		if idx := strings.LastIndex(hostport, ":"); idx > 0 && !strings.Contains(hostport[:idx], ":") {
			maybe := hostport[idx+1:]
			if port, err := strconv.Atoi(maybe); err == nil && validPort(port) {
				return hostport[:idx], port
			}
		}
	}
	if strings.Contains(hostport, ":") {
		return "", 0
	}
	return hostport, 0
}

func stripSSHPass(tokens []string) ([]string, []warning) {
	if len(tokens) == 0 || !isBinary(tokens[0], "sshpass") {
		return tokens, nil
	}
	var warnings []warning
	i := 1
	for i < len(tokens) && !isSSHBinary(tokens[i]) {
		tok := tokens[i]
		if tok == "-p" || strings.HasPrefix(tok, "-p") {
			warnings = append(warnings, warning{Code: WarnIgnoredSecret, Option: "sshpass -p"})
			if tok == "-p" {
				i += 2
				continue
			}
			i++
			continue
		}
		i++
	}
	if i >= len(tokens) {
		return nil, warnings
	}
	return tokens[i:], warnings
}

func isSSHBinary(token string) bool {
	return isBinary(token, "ssh")
}

func isBinary(token, name string) bool {
	base := strings.ReplaceAll(token, "\\", "/")
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.TrimSuffix(strings.ToLower(base), ".exe")
	return base == name
}

func safeToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) > 40 {
		token = token[:40]
	}
	if strings.ContainsAny(token, " \t") {
		return ""
	}
	return token
}

func parseNonNegInt(raw string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func parsePositiveInt(raw string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
