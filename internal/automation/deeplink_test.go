package automation

import "testing"

func TestParseLinkAcceptsActionsAndEncoding(t *testing.T) {
	cases := []struct {
		raw    string
		action string
		name   string
	}{
		{"norka://connect/db", "connect", "db"},
		{"norka://CONNECT/Db", "connect", "Db"},
		{"norka://disconnect/my%20tunnel", "disconnect", "my tunnel"},
		{"norka://open/%D0%B4%D0%BE%D0%BC", "open", "дом"},
		{"norka:///connect/db", "connect", "db"},
		{"  norka://open/db  ", "open", "db"},
	}
	for _, tc := range cases {
		link, err := ParseLink(tc.raw)
		if err != nil || link.Action != tc.action || link.Name != tc.name {
			t.Fatalf("%s: %+v %v", tc.raw, link, err)
		}
	}
	if ConnectURL("my tunnel") != "norka://connect/my%20tunnel" {
		t.Fatalf("connect url: %s", ConnectURL("my tunnel"))
	}
	round, err := ParseLink(ConnectURL("дом/нет"))
	if err == nil {
		t.Fatalf("slash in a name must not round-trip: %+v", round)
	}
	round, err = ParseLink(ConnectURL("мой туннель"))
	if err != nil || round.Name != "мой туннель" || round.Action != "connect" {
		t.Fatalf("unicode round trip: %+v %v", round, err)
	}
}

func TestParseLinkRejectsMaliciousInput(t *testing.T) {
	bad := []string{
		"",
		"http://connect/db",
		"norka://connect/../../etc/passwd",
		"norka://connect/%2e%2e/%2e%2e/etc",
		"norka://connect/%2e%2e%2f%2e%2e%2fetc",
		"norka://connect/%2e%2e",
		"norka://connect/%2e%2e%2e%2fpasswd",
		"norka://connect/..",
		"norka://connect/.",
		"norka://connect/",
		"norka://connect",
		"norka://connect/foo/bar",
		"norka://connect/foo%2fbar",
		"norka://connect/foo%5c..%5cwindows",
		"norka://connect/foo%00bar",
		"norka://connect/foo%0a",
		"norka://connect/%",
		"norka://connect/%zz",
		"norka://connect/%252e%252e",
		"norka://connect/%252e%252e%252fetc",
		"norka://user@connect/db",
		"norka://connect:8080/db",
		"norka://connect/db?x=1",
		"norka://connect/db#frag",
		"norka://evil/db",
		"norka://connect/db\x00",
		"javascript:alert(1)",
		"norka://connect/" + string(rune(0x202e)) + "db",
	}
	for _, raw := range bad {
		if _, err := ParseLink(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestLinkFromArgs(t *testing.T) {
	if got := LinkFromArgs([]string{"norka", "status"}); got != "" {
		t.Fatalf("cli arg: %q", got)
	}
	if got := LinkFromArgs([]string{`C:\norka.exe`, "norka://open/db"}); got != "norka://open/db" {
		t.Fatalf("url arg: %q", got)
	}
}
