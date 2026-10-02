package automation

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

const maxLinkName = 200

// Link is a parsed norka:// URL.
type Link struct {
	Action string
	Name   string
}

// ParseLink accepts norka://connect/<name>, norka://disconnect/<name> and
// norka://open/<name>. The name is decoded once. Path traversal, NULs,
// extra segments, and a second decode that becomes a path are rejected.
func ParseLink(raw string) (Link, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsRune(raw, 0) {
		return Link{}, fmt.Errorf("invalid norka link")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "norka") {
		return Link{}, fmt.Errorf("invalid norka link")
	}
	if parsed.User != nil || parsed.Opaque != "" || parsed.Port() != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Link{}, fmt.Errorf("invalid norka link")
	}

	action, escapedName, err := splitLink(parsed)
	if err != nil {
		return Link{}, err
	}
	switch strings.ToLower(action) {
	case "connect", "disconnect", "open":
		action = strings.ToLower(action)
	default:
		return Link{}, fmt.Errorf("invalid norka link")
	}
	if escapedName == "" || strings.ContainsAny(escapedName, `/\`) {
		return Link{}, fmt.Errorf("invalid norka link")
	}
	name, err := url.PathUnescape(escapedName)
	if err != nil || !safeLinkName(name) {
		return Link{}, fmt.Errorf("invalid norka link")
	}
	if again, err := url.PathUnescape(name); err == nil && again != name && !safeLinkName(again) {
		return Link{}, fmt.Errorf("invalid norka link")
	}
	return Link{Action: action, Name: name}, nil
}

func splitLink(parsed *url.URL) (action, escapedName string, err error) {
	escaped := parsed.EscapedPath()
	host := parsed.Hostname()
	if host != "" {
		name := strings.TrimPrefix(escaped, "/")
		if name == "" || strings.Contains(name, "/") {
			return "", "", fmt.Errorf("invalid norka link")
		}
		return host, name, nil
	}
	path := strings.TrimPrefix(escaped, "/")
	action, name, ok := strings.Cut(path, "/")
	if !ok || action == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("invalid norka link")
	}
	return action, name, nil
}

func safeLinkName(name string) bool {
	if name == "" || len(name) > maxLinkName || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// LinkFromArgs returns the first norka: argument, or an empty string.
func LinkFromArgs(args []string) string {
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if len(arg) >= 6 && strings.EqualFold(arg[:6], "norka:") {
			return arg
		}
	}
	return ""
}

// ConnectURL is the link copied from a tunnel's menu.
func ConnectURL(name string) string {
	return "norka://connect/" + url.PathEscape(name)
}
