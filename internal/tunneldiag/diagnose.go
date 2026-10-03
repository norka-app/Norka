// Package tunneldiag runs on-demand checks for one tunnel.
//
// The report is a list of stable codes and non-secret parameters. The UI
// turns those into sentences. Nothing is written to disk or the log.
// Password authentication is never attempted. Private key bytes are never
// copied into the report.
package tunneldiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"norka/internal/forward"
	"norka/internal/model"
)

const (
	dnsTimeout     = 3 * time.Second
	tcpTimeout     = 4 * time.Second
	sshTimeout     = 8 * time.Second
	bannerTimeout  = 3 * time.Second
	overallTimeout = 20 * time.Second
	maxDetailRunes = 180
)

// Check is one human-facing result. Code selects the sentence. Params fill
// its placeholders. Detail is an optional extra line, already safe to show.
type Check struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
	Detail string            `json:"detail,omitempty"`
}

// Report is the full result for one tunnel. Status is "ok" or "error".
type Report struct {
	TunnelID   int               `json:"tunnelId"`
	TunnelName string            `json:"tunnelName"`
	Status     string            `json:"status"`
	Code       string            `json:"code"`
	Params     map[string]string `json:"params,omitempty"`
	Checks     []Check           `json:"checks"`
}

// Input is the saved tunnel plus the jump hosts it uses.
// Siblings are other tunnels, used only to name a busy local port.
type Input struct {
	Tunnel   model.Tunnel
	Jumpers  []model.Jumper
	Siblings []model.Tunnel
}

// Hooks replace network calls in tests. Nil fields use the real network.
type Hooks struct {
	LookupIP    func(ctx context.Context, host string) ([]net.IP, error)
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
	Listen      func(network, address string) (net.Listener, error)
	Probe       func(ctx context.Context, jumpers []model.Jumper, targetHost string, targetPort int) (forward.ChainProbe, error)
	Banner      func(ctx context.Context, address string) (string, error)
}

// Run executes the checks. It does not persist anything.
func Run(ctx context.Context, in Input, hooks *Hooks) Report {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, overallTimeout)
	defer cancel()

	secrets := passwordList(in.Jumpers)
	tunnel := in.Tunnel
	mode := tunnelMode(tunnel.Mode)
	jumpers := normalizeJumpers(in.Jumpers)

	dnsCh := make(chan Check, 1)
	tcpCh := make(chan tcpOutcome, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		dnsCh <- dnsCheck(ctx, hooks, jumpers)
	}()
	go func() {
		defer wg.Done()
		tcpCh <- tcpCheck(ctx, hooks, jumpers)
	}()
	listen := listenCheck(hooks, tunnel, in.Siblings, mode)
	wg.Wait()
	dns := <-dnsCh
	tcp := <-tcpCh

	auth, probe := authCheck(ctx, hooks, tunnel, mode, jumpers, tcp, secrets)
	latency := latencyCheck(tcp, probe, auth)
	target := targetCheck(ctx, hooks, tunnel, mode, tcp, auth, probe)

	checks := []Check{dns, tcp.check, listen, auth, latency, target}
	report := Report{
		TunnelID:   tunnel.ID,
		TunnelName: strings.TrimSpace(tunnel.Name),
		Checks:     checks,
	}
	report.Status, report.Code, report.Params = summarize(checks)
	redactReport(&report, secrets)
	return report
}

type tcpOutcome struct {
	check   Check
	latency time.Duration
	ok      bool
}

func dnsCheck(ctx context.Context, hooks *Hooks, jumpers []model.Jumper) Check {
	if len(jumpers) == 0 {
		return Check{ID: "dns", Status: "skipped", Code: "dns_no_jumper"}
	}
	host := jumpers[0].Host
	if host == "" {
		return Check{ID: "dns", Status: "error", Code: "dns_empty"}
	}
	if literalIP(host) != nil {
		return Check{ID: "dns", Status: "skipped", Code: "dns_literal", Params: params("host", host)}
	}
	lookCtx, cancel := context.WithTimeout(ctx, dnsTimeout)
	defer cancel()
	ips, err := hooks.lookup(lookCtx, host)
	if err != nil {
		code := "dns_miss"
		if isTimeout(err) {
			code = "dns_timeout"
		}
		return Check{ID: "dns", Status: "error", Code: code, Params: params("host", host)}
	}
	text := formatIPs(ips)
	if text == "" {
		return Check{ID: "dns", Status: "error", Code: "dns_miss", Params: params("host", host)}
	}
	return Check{ID: "dns", Status: "ok", Code: "dns_ok", Params: params("host", host, "ips", text)}
}

func tcpCheck(ctx context.Context, hooks *Hooks, jumpers []model.Jumper) tcpOutcome {
	if len(jumpers) == 0 {
		return tcpOutcome{check: Check{ID: "tcp", Status: "skipped", Code: "tcp_no_jumper"}}
	}
	hop := jumpers[0]
	if hop.Host == "" {
		return tcpOutcome{check: Check{ID: "tcp", Status: "error", Code: "tcp_empty"}}
	}
	if hop.Port < 1 || hop.Port > 65535 {
		return tcpOutcome{check: Check{ID: "tcp", Status: "error", Code: "tcp_bad", Params: params("port", strconv.Itoa(hop.Port))}}
	}
	addr := net.JoinHostPort(hop.Host, strconv.Itoa(hop.Port))
	dialCtx, cancel := context.WithTimeout(ctx, tcpTimeout)
	defer cancel()
	start := time.Now()
	conn, err := hooks.dial(dialCtx, addr)
	elapsed := time.Since(start)
	if err != nil {
		code := classifyReach(err, "tcp")
		check := Check{ID: "tcp", Status: "error", Code: code, Params: params("addr", addr)}
		if code == "tcp_error" {
			check.Detail = oneLine(err, nil)
		}
		return tcpOutcome{check: check}
	}
	_ = conn.Close()
	code := "tcp_ok"
	if len(jumpers) > 1 {
		code = "tcp_ok_chain"
	}
	return tcpOutcome{
		check:   Check{ID: "tcp", Status: "ok", Code: code, Params: params("addr", addr)},
		latency: elapsed,
		ok:      true,
	}
}

func listenCheck(hooks *Hooks, tunnel model.Tunnel, siblings []model.Tunnel, mode string) Check {
	if mode == "remote" {
		return Check{ID: "listen", Status: "skipped", Code: "listen_remote"}
	}
	addr, ok := localEndpoint(tunnel)
	if !ok {
		return Check{ID: "listen", Status: "error", Code: "listen_bad", Params: params("port", strconv.Itoa(tunnel.LocalPort))}
	}
	ln, err := hooks.listen(addr)
	if err == nil {
		_ = ln.Close()
		return Check{ID: "listen", Status: "ok", Code: "listen_free", Params: params("addr", addr)}
	}
	if !isAddrInUse(err) {
		return Check{
			ID:     "listen",
			Status: "error",
			Code:   "listen_error",
			Params: params("addr", addr),
			Detail: oneLine(err, nil),
		}
	}
	if holdsLocalPort(tunnel.Status) {
		return Check{ID: "listen", Status: "ok", Code: "listen_self", Params: params("addr", addr)}
	}
	if other, found := portHolder(tunnel, siblings); found {
		name := strings.TrimSpace(other.Name)
		if name == "" {
			name = strconv.Itoa(other.ID)
		}
		return Check{ID: "listen", Status: "error", Code: "listen_other", Params: params("addr", addr, "name", name)}
	}
	return Check{ID: "listen", Status: "error", Code: "listen_busy", Params: params("addr", addr)}
}

func authCheck(ctx context.Context, hooks *Hooks, tunnel model.Tunnel, mode string, jumpers []model.Jumper, tcp tcpOutcome, secrets []string) (Check, forward.ChainProbe) {
	if len(jumpers) == 0 {
		return Check{ID: "auth", Status: "error", Code: "auth_no_jumper"}, forward.ChainProbe{}
	}
	hop := jumpers[0]
	if hop.Host == "" {
		return Check{ID: "auth", Status: "error", Code: "auth_no_host"}, forward.ChainProbe{}
	}
	if !tcp.ok {
		return Check{ID: "auth", Status: "skipped", Code: "auth_skipped_down"}, forward.ChainProbe{}
	}
	addr := net.JoinHostPort(hop.Host, strconv.Itoa(hop.Port))
	safe, blocked, blockedAt := safePrefix(jumpers)
	if len(safe) == 0 {
		return passwordBanner(ctx, hooks, addr, hop.Host), forward.ChainProbe{}
	}
	for _, item := range safe {
		if problem, ok := keyFileIssue(item, secrets); !ok {
			return problem, forward.ChainProbe{}
		}
		if !knownAuth(item.AuthType) {
			return Check{ID: "auth", Status: "error", Code: "auth_unsupported", Params: params("host", item.Host)}, forward.ChainProbe{}
		}
	}

	// Dial the tunnel destination in the same login when the whole chain can
	// be probed. A later password hop never reaches this call.
	targetHost, targetPort := "", 0
	if blockedAt < 0 && mode == "local" {
		if host, port, ok := remoteEndpoint(tunnel); ok {
			targetHost, targetPort = host, port
		}
	}
	probe, err := hooks.probe(ctx, safe, targetHost, targetPort)
	if err != nil {
		return authFailure(err, safe, secrets), forward.ChainProbe{}
	}
	last := safe[len(safe)-1]
	if blockedAt >= 0 {
		next := blocked.Host
		if next == "" {
			next = "—"
		}
		return Check{
			ID:     "auth",
			Status: "skipped",
			Code:   "auth_prefix_ok",
			Params: params("user", displayUser(last.User), "host", last.Host, "next", next),
		}, probe
	}
	return Check{
		ID:     "auth",
		Status: "ok",
		Code:   "auth_ok",
		Params: params("user", displayUser(last.User), "host", last.Host),
	}, probe
}

func passwordBanner(ctx context.Context, hooks *Hooks, addr, host string) Check {
	bannerCtx, cancel := context.WithTimeout(ctx, bannerTimeout)
	defer cancel()
	banner, err := hooks.banner(bannerCtx, addr)
	if err != nil {
		code := "auth_banner_bad"
		if isTimeout(err) {
			code = "auth_timeout"
		}
		paramsKey := "addr"
		if code == "auth_timeout" {
			paramsKey = "host"
			addr = host
		}
		return Check{ID: "auth", Status: "error", Code: code, Params: params(paramsKey, addr)}
	}
	return Check{
		ID:     "auth",
		Status: "skipped",
		Code:   "auth_skipped_password",
		Params: params("host", host),
		Detail: banner,
	}
}

func authFailure(err error, safe []model.Jumper, secrets []string) Check {
	last := safe[len(safe)-1]
	host := last.Host
	user := displayUser(last.User)
	if errors.Is(err, forward.ErrPasswordNotProbed) {
		return Check{ID: "auth", Status: "skipped", Code: "auth_skipped_password", Params: params("host", host)}
	}
	if isTimeout(err) {
		return Check{ID: "auth", Status: "error", Code: "auth_timeout", Params: params("host", host)}
	}
	if isHostKey(err) {
		return Check{ID: "auth", Status: "error", Code: "auth_hostkey", Params: params("host", host)}
	}
	if errors.Is(err, os.ErrNotExist) {
		return Check{ID: "auth", Status: "error", Code: "auth_key_missing", Params: params("file", filepath.Base(last.KeyPath))}
	}
	if isAuthRejected(err) {
		return Check{ID: "auth", Status: "error", Code: "auth_rejected", Params: params("user", user, "host", host)}
	}
	if isKeyEncrypted(err) {
		file := filepath.Base(strings.TrimSpace(last.KeyPath))
		if file == "." || file == "" {
			file = "key"
		}
		return Check{ID: "auth", Status: "error", Code: "auth_key_encrypted", Params: params("file", file)}
	}
	if addr, step, ok := hopFailure(err); ok {
		return Check{
			ID:     "auth",
			Status: "error",
			Code:   "auth_hop",
			Params: params("hop", strconv.Itoa(step), "addr", addr),
			Detail: oneLine(err, secrets),
		}
	}
	return Check{
		ID:     "auth",
		Status: "error",
		Code:   "auth_error",
		Params: params("host", host),
		Detail: oneLine(err, secrets),
	}
}

func latencyCheck(tcp tcpOutcome, probe forward.ChainProbe, auth Check) Check {
	if probe.Latency > 0 && (auth.Code == "auth_ok" || auth.Code == "auth_prefix_ok") {
		return Check{ID: "latency", Status: "ok", Code: "latency_ssh", Params: params("ms", formatMS(probe.Latency))}
	}
	if tcp.ok {
		return Check{ID: "latency", Status: "ok", Code: "latency_tcp", Params: params("ms", formatMS(tcp.latency))}
	}
	return Check{ID: "latency", Status: "skipped", Code: "latency_skipped"}
}

func targetCheck(ctx context.Context, hooks *Hooks, tunnel model.Tunnel, mode string, tcp tcpOutcome, auth Check, probe forward.ChainProbe) Check {
	switch mode {
	case "dynamic":
		return Check{ID: "target", Status: "skipped", Code: "target_skipped_dynamic"}
	case "remote":
		return localServiceCheck(ctx, hooks, tunnel)
	default:
		return forwardTargetCheck(tunnel, tcp, auth, probe)
	}
}

func localServiceCheck(ctx context.Context, hooks *Hooks, tunnel model.Tunnel) Check {
	addr, ok := localEndpoint(tunnel)
	if !ok {
		return Check{ID: "target", Status: "error", Code: "target_bad"}
	}
	dialCtx, cancel := context.WithTimeout(ctx, tcpTimeout)
	defer cancel()
	conn, err := hooks.dial(dialCtx, addr)
	if err != nil {
		code := "target_local_error"
		switch {
		case isRefused(err):
			code = "target_local_refused"
		case isTimeout(err):
			code = "target_local_timeout"
		}
		check := Check{ID: "target", Status: "error", Code: code, Params: params("addr", addr)}
		if code == "target_local_error" {
			check.Detail = oneLine(err, nil)
		}
		return check
	}
	_ = conn.Close()
	return Check{ID: "target", Status: "ok", Code: "target_local_ok", Params: params("addr", addr)}
}

func forwardTargetCheck(tunnel model.Tunnel, tcp tcpOutcome, auth Check, probe forward.ChainProbe) Check {
	host, port, ok := remoteEndpoint(tunnel)
	if !ok {
		return Check{ID: "target", Status: "error", Code: "target_bad"}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	if !tcp.ok {
		return Check{ID: "target", Status: "skipped", Code: "target_skipped_down"}
	}
	if auth.Code != "auth_ok" {
		return Check{ID: "target", Status: "skipped", Code: "target_skipped_auth"}
	}
	if !probe.TargetChecked {
		return Check{ID: "target", Status: "error", Code: "target_error", Params: params("addr", addr)}
	}
	return targetFromErr(probe.TargetErr, addr)
}

func targetFromErr(err error, addr string) Check {
	if err == nil {
		return Check{ID: "target", Status: "ok", Code: "target_ok", Params: params("addr", addr)}
	}
	if isForwardDenied(err) {
		return Check{ID: "target", Status: "error", Code: "target_denied", Params: params("addr", addr)}
	}
	code := "target_error"
	switch {
	case isRefused(err):
		code = "target_refused"
	case isTimeout(err):
		code = "target_timeout"
	case isUnreachable(err):
		code = "target_unreachable"
	}
	check := Check{ID: "target", Status: "error", Code: code, Params: params("addr", addr)}
	if code == "target_error" {
		check.Detail = oneLine(err, nil)
	}
	return check
}

func summarize(checks []Check) (string, string, map[string]string) {
	problems := 0
	passwordSkip := false
	for _, check := range checks {
		if check.Status == "error" {
			problems++
		}
		if check.Code == "auth_skipped_password" || check.Code == "auth_prefix_ok" {
			passwordSkip = true
		}
	}
	if problems > 0 {
		return "error", "summary_error", params("count", strconv.Itoa(problems))
	}
	if passwordSkip {
		return "ok", "summary_password", nil
	}
	return "ok", "summary_ok", nil
}

func keyFileIssue(hop model.Jumper, secrets []string) (Check, bool) {
	if hop.AuthType != "ssh_key" {
		return Check{}, true
	}
	raw := strings.TrimSpace(hop.KeyPath)
	if raw == "" {
		return Check{ID: "auth", Status: "error", Code: "auth_key_missing_empty"}, false
	}
	resolved, err := forward.ResolveKeyPath(raw)
	if err != nil {
		return Check{ID: "auth", Status: "error", Code: "auth_key_missing_empty"}, false
	}
	info, statErr := os.Stat(resolved)
	file := filepath.Base(resolved)
	detail := redact(resolved, secrets)
	if statErr != nil {
		code := "auth_key_unreadable"
		if errors.Is(statErr, os.ErrNotExist) {
			code = "auth_key_missing"
		}
		return Check{ID: "auth", Status: "error", Code: code, Params: params("file", file), Detail: detail}, false
	}
	if info.IsDir() {
		return Check{ID: "auth", Status: "error", Code: "auth_key_unreadable", Params: params("file", file), Detail: detail}, false
	}
	f, openErr := os.Open(resolved)
	if openErr != nil {
		return Check{ID: "auth", Status: "error", Code: "auth_key_unreadable", Params: params("file", file), Detail: detail}, false
	}
	_ = f.Close()
	return Check{}, true
}

func knownAuth(authType string) bool {
	switch authType {
	case "ssh_key", "ssh_agent":
		return true
	default:
		return false
	}
}

// safePrefix returns the leading key/agent hops and the first password hop.
func safePrefix(jumpers []model.Jumper) (safe []model.Jumper, blocked model.Jumper, blockedIndex int) {
	for i, hop := range jumpers {
		if hop.AuthType == "password" {
			return jumpers[:i:i], hop, i
		}
	}
	return jumpers, model.Jumper{}, -1
}

func normalizeJumpers(items []model.Jumper) []model.Jumper {
	out := make([]model.Jumper, 0, len(items))
	for _, item := range items {
		item.Host = strings.TrimSpace(item.Host)
		item.User = strings.TrimSpace(item.User)
		item.AuthType = strings.TrimSpace(item.AuthType)
		item.KeyPath = strings.TrimSpace(item.KeyPath)
		if item.Port <= 0 {
			item.Port = 22
		}
		out = append(out, item)
	}
	return out
}

func tunnelMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "local"
	}
	return mode
}

func localEndpoint(tunnel model.Tunnel) (string, bool) {
	if tunnel.LocalPort < 1 || tunnel.LocalPort > 65535 {
		return "", false
	}
	host := strings.TrimSpace(tunnel.LocalHost)
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(tunnel.LocalPort)), true
}

func remoteEndpoint(tunnel model.Tunnel) (string, int, bool) {
	host := strings.TrimSpace(tunnel.RemoteHost)
	if host == "" || tunnel.RemotePort < 1 || tunnel.RemotePort > 65535 {
		return "", 0, false
	}
	return host, tunnel.RemotePort, true
}

func holdsLocalPort(status string) bool {
	switch strings.TrimSpace(status) {
	case "running", "reconnecting":
		return true
	default:
		return false
	}
}

func portHolder(self model.Tunnel, siblings []model.Tunnel) (model.Tunnel, bool) {
	addr, ok := localEndpoint(self)
	if !ok {
		return model.Tunnel{}, false
	}
	for _, other := range siblings {
		if other.ID == self.ID || tunnelMode(other.Mode) == "remote" {
			continue
		}
		otherAddr, ok := localEndpoint(other)
		if !ok || otherAddr != addr || !holdsLocalPort(other.Status) {
			continue
		}
		return other, true
	}
	return model.Tunnel{}, false
}

func literalIP(host string) net.IP {
	return net.ParseIP(strings.Trim(host, "[]"))
}

func formatIPs(ips []net.IP) string {
	parts := make([]string, 0, 4)
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		parts = append(parts, ip.String())
		if len(parts) == 4 {
			break
		}
	}
	text := strings.Join(parts, ", ")
	if extra := len(ips) - len(parts); extra > 0 && text != "" {
		text += fmt.Sprintf(" +%d", extra)
	}
	return text
}

func displayUser(user string) string {
	user = strings.TrimSpace(user)
	if user == "" {
		return "—"
	}
	return user
}

func formatMS(d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	ms := d.Milliseconds()
	if ms < 1 {
		return "1"
	}
	return strconv.FormatInt(ms, 10)
}

func classifyReach(err error, prefix string) string {
	switch {
	case isRefused(err):
		return prefix + "_refused"
	case isTimeout(err):
		return prefix + "_timeout"
	case isUnreachable(err):
		return prefix + "_unreachable"
	default:
		return prefix + "_error"
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out") || strings.Contains(msg, "deadline exceeded")
}

func isRefused(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "actively refused")
}

func isUnreachable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "host is unreachable") ||
		strings.Contains(msg, "host is down")
}

func isHostKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "knownhosts") || strings.Contains(msg, "host key") || strings.Contains(msg, "key mismatch")
}

func isAuthRejected(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods") ||
		strings.Contains(msg, "permission denied")
}

func isKeyEncrypted(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "passphrase") || strings.Contains(msg, "encrypted") || strings.Contains(msg, "parse key")
}

func isForwardDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "administratively prohibited") ||
		strings.Contains(msg, "forwarding disabled") ||
		strings.Contains(msg, "port forwarding disabled")
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage of each socket address")
}

var viaHopPattern = regexp.MustCompile(`ssh (?:dial|handshake) (\S+) via hop (\d+)`)

func hopFailure(err error) (string, int, bool) {
	if err == nil {
		return "", 0, false
	}
	match := viaHopPattern.FindStringSubmatch(err.Error())
	if match == nil {
		return "", 0, false
	}
	n, convErr := strconv.Atoi(match[2])
	if convErr != nil {
		return "", 0, false
	}
	return match[1], n + 1, true
}

func (h *Hooks) lookup(ctx context.Context, host string) ([]net.IP, error) {
	if h != nil && h.LookupIP != nil {
		return h.LookupIP(ctx, host)
	}
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

func (h *Hooks) dial(ctx context.Context, address string) (net.Conn, error) {
	if h != nil && h.DialContext != nil {
		return h.DialContext(ctx, "tcp", address)
	}
	return (&net.Dialer{Timeout: tcpTimeout}).DialContext(ctx, "tcp", address)
}

func (h *Hooks) listen(address string) (net.Listener, error) {
	if h != nil && h.Listen != nil {
		return h.Listen("tcp", address)
	}
	return net.Listen("tcp", address)
}

func (h *Hooks) probe(ctx context.Context, jumpers []model.Jumper, targetHost string, targetPort int) (forward.ChainProbe, error) {
	if h != nil && h.Probe != nil {
		return h.Probe(ctx, jumpers, targetHost, targetPort)
	}
	probeCtx, cancel := context.WithTimeout(ctx, sshTimeout)
	defer cancel()
	return forward.ProbeChain(probeCtx, jumpers, targetHost, targetPort)
}

func (h *Hooks) banner(ctx context.Context, address string) (string, error) {
	if h != nil && h.Banner != nil {
		return h.Banner(ctx, address)
	}
	return readSSHBanner(ctx, address)
}

func readSSHBanner(ctx context.Context, address string) (string, error) {
	conn, err := (&net.Dialer{Timeout: bannerTimeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	deadline := time.Now().Add(bannerTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
	buf := make([]byte, 255)
	n, err := conn.Read(buf)
	if n == 0 && err != nil {
		return "", err
	}
	line := string(buf[:n])
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "SSH-") {
		return "", errors.New("not an ssh banner")
	}
	token := sanitizeBanner(line)
	if token == "" {
		return "", errors.New("not an ssh banner")
	}
	return token, nil
}

func sanitizeBanner(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	token := fields[0]
	if len(token) > 64 {
		token = token[:64]
	}
	for _, r := range token {
		if r < 32 || r > 126 {
			return ""
		}
	}
	return token
}

func params(pairs ...string) map[string]string {
	if len(pairs) == 0 || len(pairs)%2 != 0 {
		return nil
	}
	out := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out[pairs[i]] = pairs[i+1]
	}
	return out
}

func oneLine(err error, secrets []string) string {
	if err == nil {
		return ""
	}
	text := redact(err.Error(), secrets)
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > maxDetailRunes {
		text = string(runes[:maxDetailRunes]) + "…"
	}
	return text
}

var pemPattern = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)

func redact(text string, secrets []string) string {
	if text == "" {
		return ""
	}
	text = pemPattern.ReplaceAllString(text, "••••")
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if len(secret) < 4 || !strings.Contains(text, secret) {
			continue
		}
		if len(secret) >= 8 {
			text = strings.ReplaceAll(text, secret, "••••")
			continue
		}
		text = replaceBounded(text, secret, "••••")
	}
	return text
}

func replaceBounded(text, secret, repl string) string {
	var b strings.Builder
	rest := text
	for {
		i := strings.Index(rest, secret)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		end := i + len(secret)
		beforeOK := i == 0 || !isSecretBoundary(rest[i-1])
		afterOK := end >= len(rest) || !isSecretBoundary(rest[end])
		b.WriteString(rest[:i])
		if beforeOK && afterOK {
			b.WriteString(repl)
		} else {
			b.WriteString(secret)
		}
		rest = rest[end:]
	}
}

func isSecretBoundary(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func redactReport(report *Report, secrets []string) {
	if report == nil {
		return
	}
	report.TunnelName = redact(report.TunnelName, secrets)
	for key, value := range report.Params {
		report.Params[key] = redact(value, secrets)
	}
	for i := range report.Checks {
		report.Checks[i].Detail = redact(report.Checks[i].Detail, secrets)
		for key, value := range report.Checks[i].Params {
			report.Checks[i].Params[key] = redact(value, secrets)
		}
	}
}

func passwordList(jumpers []model.Jumper) []string {
	out := make([]string, 0, len(jumpers))
	seen := make(map[string]struct{}, len(jumpers))
	for _, hop := range jumpers {
		secret := strings.TrimSpace(hop.Password)
		if len(secret) < 4 {
			continue
		}
		if _, ok := seen[secret]; ok {
			continue
		}
		seen[secret] = struct{}{}
		out = append(out, secret)
	}
	return out
}
