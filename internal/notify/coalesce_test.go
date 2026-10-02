package notify

import (
	"strings"
	"testing"
	"time"
)

func TestCoalesceSuppressesFlapAndAggregates(t *testing.T) {
	events := []Event{
		{Kind: KindDropped, TunnelID: 1, TunnelName: "db"},
		{Kind: KindReconnected, TunnelID: 1, TunnelName: "db"},
		{Kind: KindReconnected, TunnelID: 2, TunnelName: "web"},
		{Kind: KindReconnected, TunnelID: 3, TunnelName: "cache"},
		{Kind: KindDropped, TunnelID: 4, TunnelName: "logs"},
	}
	groups := Coalesce(events)
	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Kind != KindDropped || len(groups[0].Tunnels) != 1 || groups[0].Tunnels[0].ID != 4 {
		t.Fatalf("dropped group = %+v", groups[0])
	}
	if groups[1].Kind != KindReconnected || len(groups[1].Tunnels) != 3 {
		t.Fatalf("reconnected group = %+v", groups[1])
	}
}

func TestFormatRussianPlurals(t *testing.T) {
	cat := CatalogFor("ru")
	one := Format(cat, Group{Kind: KindReconnected, Tunnels: []TunnelRef{{ID: 1, Name: "db"}}})
	if one.Body != "Туннель «db» переподключён" || one.TunnelID != 1 {
		t.Fatalf("one = %+v", one)
	}
	three := Format(cat, Group{Kind: KindReconnected, Tunnels: []TunnelRef{{ID: 1}, {ID: 2}, {ID: 3}}})
	if three.Body != "3 туннеля переподключены" || three.TunnelID != 0 {
		t.Fatalf("three = %+v", three)
	}
	five := Format(cat, Group{Kind: KindDropped, Tunnels: make([]TunnelRef, 5)})
	if five.Body != "5 туннелей отключились" {
		t.Fatalf("five = %+v", five)
	}
	eleven := Format(cat, Group{Kind: KindGaveUp, Tunnels: make([]TunnelRef, 11)})
	if eleven.Body != "Не удалось восстановить 11 туннелей" {
		t.Fatalf("eleven = %+v", eleven)
	}
	twentyOne := Format(cat, Group{Kind: KindConnected, Tunnels: make([]TunnelRef, 21)})
	if twentyOne.Body != "21 туннель подключён" {
		t.Fatalf("twentyOne = %+v", twentyOne)
	}
}

func TestFormatEnglish(t *testing.T) {
	cat := CatalogFor("en")
	one := Format(cat, Group{Kind: KindDropped, Tunnels: []TunnelRef{{ID: 4, Name: "db"}}})
	if one.Body != `Tunnel "db" disconnected` || one.TunnelID != 4 {
		t.Fatalf("one = %+v", one)
	}
	three := Format(cat, Group{Kind: KindReconnected, Tunnels: []TunnelRef{{ID: 1}, {ID: 2}, {ID: 3}}})
	if three.Body != "3 tunnels reconnected" || three.TunnelID != 0 {
		t.Fatalf("three = %+v", three)
	}
	failed := Format(cat, Group{Kind: KindConnectFailed, Tunnels: []TunnelRef{{ID: 2, Name: "web"}}})
	if failed.Body != `Failed to connect tunnel "web"` {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestServiceDebounceAndMasterSwitch(t *testing.T) {
	var mu = struct {
		notices []Notice
	}{}
	poster := posterFunc(func(n Notice) error {
		mu.notices = append(mu.notices, n)
		return nil
	})
	settings := Settings{Enabled: false, Reconnected: true, Dropped: true}
	svc := NewService(ServiceConfig{
		Window:   time.Hour,
		Settings: func() Settings { return settings },
		Catalog:  func() Catalog { return CatalogFor("ru") },
		Poster:   poster,
	})
	svc.Reconnected(1, "db")
	svc.FlushNow()
	if len(mu.notices) != 0 {
		t.Fatalf("disabled master posted %#v", mu.notices)
	}

	settings.Enabled = true
	svc.Dropped(1, "db")
	svc.Reconnected(1, "db")
	svc.Reconnected(2, "web")
	svc.Reconnected(3, "cache")
	svc.FlushNow()
	if len(mu.notices) != 1 {
		t.Fatalf("notices = %#v", mu.notices)
	}
	if !strings.Contains(mu.notices[0].Body, "3 туннеля переподключены") {
		t.Fatalf("body = %q", mu.notices[0].Body)
	}
}

func TestParseFocusArg(t *testing.T) {
	if got := ParseFocusArg([]string{"norka", "--norka-focus=12"}); got != 12 {
		t.Fatalf("got %d", got)
	}
	if got := ParseFocusArg([]string{"--norka-focus=0", "-Embedding"}); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := ParseFocusArg([]string{"app.exe --norka-focus=4"}); got != 4 {
		t.Fatalf("got %d", got)
	}
}

type posterFunc func(Notice) error

func (f posterFunc) Post(n Notice) error { return f(n) }
