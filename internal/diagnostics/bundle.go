package diagnostics

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"norka/internal/conf"
)

// IssuesNewURL is the GitHub form that opens with a prefilled report.
const IssuesNewURL = "https://github.com/norka-app/Norka/issues/new"

const readmeText = `Norka diagnostics
=================

Этот архив помогает сообщить об ошибке.
This archive helps report a bug.

Что внутри / What's inside
- README.txt — этот файл / this file
- system.txt — версия, система, локаль, тема и включённые функции / version, system, locale, theme, and enabled features
- config.toml — конфигурация без секретов: порты, режимы и флаги / configuration without secrets: ports, modes, and flags
- tunnels.txt — состояние туннелей и последние ошибки / tunnel states and recent errors
- norka.log — хвост журнала, не больше 2 МБ / log tail, at most 2 MB
- stats.json — суммы статистики, если файл уже был / totals, when stats.json already existed

Секретов нет / No secrets
В архиве нет паролей, passphrase, закрытых ключей, содержимого файлов ключей, данных связки ключей и токенов, включая токен автоматизации.
The archive does not include passwords, passphrases, private keys, key file contents, keychain data, or tokens, including the automation token.

Адреса серверов и имена пользователей заменены на ***, если при сохранении не была включена опция «Включить адреса серверов».
Server addresses and usernames are replaced with *** unless “Include server addresses” was enabled when the archive was saved.

Приложите архив к issue, если его попросят. Не дописывайте пароли вручную.
Attach the archive to an issue if asked. Do not add passwords by hand.
`

// SystemInfo is the host and build description written to system.txt.
type SystemInfo struct {
	Version        string
	Commit         string
	OS             string
	Arch           string
	OSVersion      string
	GoVersion      string
	Locale         string
	Theme          string
	WailsVersion   string
	WebViewVersion string
	Flags          []FlagState
}

// FlagState is one feature flag as it is in effect, not only the explicit key.
type FlagState struct {
	ID      string
	Enabled bool
}

// TunnelState is a tunnel's runtime status. Hosts are not copied here.
type TunnelState struct {
	ID         int
	Name       string
	Mode       string
	Status     string
	LocalPort  int
	RemotePort int
	AutoStart  bool
	LastError  string
}

// StatTotals is the persisted counters for one tunnel.
type StatTotals struct {
	ID                int
	TodayConnectedSec int64
	TotalConnectedSec int64
	ReconnectsToday   int
	ReconnectsTotal   int
	TotalBytesUp      uint64
	TotalBytesDown    uint64
	LastError         string
}

// Input is everything the archive is allowed to see. Key files are not read.
type Input struct {
	Now          time.Time
	IncludeHosts bool
	System       SystemInfo
	Config       *conf.Config
	Log          []byte
	Tunnels      []TunnelState
	Stats        []StatTotals
	StatsPresent bool
	// Secrets are extra values to cut out, such as the automation token.
	// They are never written.
	Secrets []string
}

// File is one archive entry. Name is a base name, never a path.
type File struct {
	Name string
	Data []byte
}

// Bundle is the archive contents and the system text used for a GitHub issue.
type Bundle struct {
	SystemText string
	Files      []File
	IssueURL   string
}

// Filename is the default save-dialog name in the user's local time.
func Filename(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.Format("norka-diagnostics-20060102-1504.zip")
}

// Build redacts the config and assembles the archive files.
func Build(in Input) (Bundle, error) {
	redacted, list := redactConfig(in.Config, in.IncludeHosts, in.Secrets)
	idents := list.identities(in.IncludeHosts)

	configText := scrubText(string(conf.MarshalTOML(redacted)), list.secrets, idents)
	system := scrubText(renderSystem(in.System), list.secrets, nil)
	tunnels := scrubText(renderTunnels(in.Tunnels), list.secrets, idents)
	logText := scrubText(string(Tail(in.Log, MaxLogBytes, MaxLogLines)), list.secrets, idents)

	files := []File{
		{Name: "README.txt", Data: []byte(readmeText)},
		{Name: "system.txt", Data: []byte(system)},
		{Name: "config.toml", Data: []byte(configText)},
		{Name: "tunnels.txt", Data: []byte(tunnels)},
		{Name: "norka.log", Data: []byte(logText)},
	}
	if in.StatsPresent {
		raw, err := renderStats(in.Stats)
		if err != nil {
			return Bundle{}, err
		}
		files = append(files, File{Name: "stats.json", Data: []byte(scrubText(string(raw), list.secrets, idents))})
	}
	return Bundle{
		SystemText: system,
		Files:      files,
		IssueURL:   IssueURL(system),
	}, nil
}

// WriteZip writes files in order. Names stay inside the archive root.
func WriteZip(w io.Writer, files []File, modified time.Time) error {
	if modified.IsZero() {
		modified = time.Now().UTC()
	}
	zw := zip.NewWriter(w)
	for _, file := range files {
		name := strings.TrimSpace(file.Name)
		if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) || name == ".." {
			return fmt.Errorf("diagnostics entry %q is not a file name", file.Name)
		}
		header := &zip.FileHeader{
			Name:     name,
			Method:   zip.Deflate,
			Modified: modified.UTC(),
		}
		header.SetMode(0o644)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := writer.Write(file.Data); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func renderSystem(info SystemInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "version: %s\n", orUnavailable(info.Version))
	fmt.Fprintf(&b, "commit: %s\n", orUnavailable(info.Commit))
	fmt.Fprintf(&b, "os: %s\n", orUnavailable(info.OS))
	fmt.Fprintf(&b, "arch: %s\n", orUnavailable(info.Arch))
	fmt.Fprintf(&b, "os_version: %s\n", orUnavailable(info.OSVersion))
	fmt.Fprintf(&b, "go: %s\n", orUnavailable(info.GoVersion))
	fmt.Fprintf(&b, "locale: %s\n", orUnavailable(info.Locale))
	fmt.Fprintf(&b, "theme: %s\n", NormalizeTheme(info.Theme))
	fmt.Fprintf(&b, "wails: %s\n", orUnavailable(info.WailsVersion))
	fmt.Fprintf(&b, "webview: %s\n", orUnavailable(info.WebViewVersion))
	b.WriteString("flags:\n")
	for _, flag := range info.Flags {
		id := strings.TrimSpace(flag.ID)
		if id == "" {
			continue
		}
		fmt.Fprintf(&b, "%s=%t\n", id, flag.Enabled)
	}
	return b.String()
}

func renderTunnels(tunnels []TunnelState) string {
	if len(tunnels) == 0 {
		return "# no tunnels\n"
	}
	var b strings.Builder
	for _, tunnel := range tunnels {
		fmt.Fprintf(&b, "id=%d name=%s mode=%s status=%s local_port=%d remote_port=%d auto_start=%t\n",
			tunnel.ID, oneLine(tunnel.Name), oneLine(tunnel.Mode), oneLine(tunnel.Status),
			tunnel.LocalPort, tunnel.RemotePort, tunnel.AutoStart)
		if strings.TrimSpace(tunnel.LastError) != "" {
			fmt.Fprintf(&b, "error: %s\n", oneLine(tunnel.LastError))
		}
	}
	return b.String()
}

type statsDocument struct {
	Tunnels []statsTunnel `json:"tunnels"`
}

type statsTunnel struct {
	ID                int    `json:"id"`
	TodayConnectedSec int64  `json:"todayConnectedSec"`
	TotalConnectedSec int64  `json:"totalConnectedSec"`
	ReconnectsToday   int    `json:"reconnectsToday"`
	ReconnectsTotal   int    `json:"reconnectsTotal"`
	TotalBytesUp      uint64 `json:"totalBytesUp"`
	TotalBytesDown    uint64 `json:"totalBytesDown"`
	LastError         string `json:"lastError,omitempty"`
}

func renderStats(stats []StatTotals) ([]byte, error) {
	doc := statsDocument{Tunnels: make([]statsTunnel, 0, len(stats))}
	for _, stat := range stats {
		doc.Tunnels = append(doc.Tunnels, statsTunnel{
			ID:                stat.ID,
			TodayConnectedSec: stat.TodayConnectedSec,
			TotalConnectedSec: stat.TotalConnectedSec,
			ReconnectsToday:   stat.ReconnectsToday,
			ReconnectsTotal:   stat.ReconnectsTotal,
			TotalBytesUp:      stat.TotalBytesUp,
			TotalBytesDown:    stat.TotalBytesDown,
			LastError:         oneLine(stat.LastError),
		})
	}
	return json.MarshalIndent(doc, "", "  ")
}

func oneLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func orUnavailable(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unavailable"
	}
	return value
}

// NormalizeTheme keeps the two themes the interface actually stores.
func NormalizeTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "light", "dark":
		return strings.ToLower(strings.TrimSpace(theme))
	default:
		return "unknown"
	}
}

// IssueURL opens a GitHub issue whose body is system.txt and a short note.
// The zip is not attached. Secrets are not added here; system.txt is already redacted.
func IssueURL(systemText string) string {
	systemText = strings.TrimSpace(systemText)
	title := issueTitle(systemText)
	body := issueBody(systemText)
	for len(issuesURL(title, body)) > 7000 && len(systemText) > 400 {
		systemText = strings.TrimSpace(systemText[:len(systemText)-200])
		body = issueBody(systemText + "\n…")
	}
	return issuesURL(title, body)
}

func issueTitle(systemText string) string {
	version := systemValue(systemText, "version")
	if version == "" || version == "unavailable" {
		return "Norka diagnostics"
	}
	return "Norka " + version + " diagnostics"
}

func issueBody(systemText string) string {
	return "Опишите, что случилось.\nDescribe what went wrong.\n\n" +
		"Приложите zip из «Собрать диагностику». В нём нет паролей, ключей и токенов.\n" +
		"Attach the zip from Collect diagnostics. It contains no passwords, keys, or tokens.\n\n" +
		"```\n" + systemText + "\n```\n"
}

func issuesURL(title, body string) string {
	query := url.Values{}
	query.Set("title", title)
	query.Set("body", body)
	return IssuesNewURL + "?" + query.Encode()
}

func systemValue(systemText, key string) string {
	prefix := key + ": "
	for _, line := range strings.Split(systemText, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// ZipBytes is a test helper that returns the archive bytes.
func ZipBytes(files []File, modified time.Time) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteZip(&buf, files, modified); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
