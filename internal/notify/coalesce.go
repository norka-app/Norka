package notify

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind is one user-visible tunnel notification.
type Kind string

const (
	KindDropped       Kind = "dropped"
	KindReconnected   Kind = "reconnected"
	KindGaveUp        Kind = "gave_up"
	KindConnectFailed Kind = "connect_failed"
	KindConnected     Kind = "connected"
)

// Event is a single tunnel transition before aggregation.
type Event struct {
	Kind       Kind
	TunnelID   int
	TunnelName string
}

// TunnelRef is one tunnel inside an aggregated notification.
type TunnelRef struct {
	ID   int
	Name string
}

// Group is one notification after debounce.
type Group struct {
	Kind    Kind
	Tunnels []TunnelRef
}

// Notice is the text handed to the OS.
type Notice struct {
	Title    string
	Body     string
	TunnelID int
}

// Settings is the opt-in notification policy.
type Settings struct {
	Enabled       bool
	Dropped       bool
	Reconnected   bool
	GaveUp        bool
	ConnectFailed bool
	Connected     bool
}

// Allows reports whether this kind should be shown.
func (s Settings) Allows(kind Kind) bool {
	if !s.Enabled {
		return false
	}
	switch kind {
	case KindDropped:
		return s.Dropped
	case KindReconnected:
		return s.Reconnected
	case KindGaveUp:
		return s.GaveUp
	case KindConnectFailed:
		return s.ConnectFailed
	case KindConnected:
		return s.Connected
	default:
		return false
	}
}

// Coalesce collapses a burst of events.
// A tunnel that both drops and reconnects in the same burst keeps only the later event.
func Coalesce(events []Event) []Group {
	order := []Kind{KindDropped, KindReconnected, KindGaveUp, KindConnectFailed, KindConnected}
	net := map[int]Event{}
	var netOrder []int
	type otherKey struct {
		kind Kind
		id   int
	}
	others := map[otherKey]Event{}
	var otherOrder []otherKey

	for _, ev := range events {
		switch ev.Kind {
		case KindDropped, KindReconnected:
			if _, ok := net[ev.TunnelID]; !ok {
				netOrder = append(netOrder, ev.TunnelID)
			}
			net[ev.TunnelID] = ev
		default:
			key := otherKey{kind: ev.Kind, id: ev.TunnelID}
			if _, ok := others[key]; !ok {
				otherOrder = append(otherOrder, key)
			}
			others[key] = ev
		}
	}

	grouped := map[Kind][]TunnelRef{}
	for _, id := range netOrder {
		ev := net[id]
		grouped[ev.Kind] = append(grouped[ev.Kind], TunnelRef{ID: ev.TunnelID, Name: ev.TunnelName})
	}
	for _, key := range otherOrder {
		ev := others[key]
		grouped[ev.Kind] = append(grouped[ev.Kind], TunnelRef{ID: ev.TunnelID, Name: ev.TunnelName})
	}

	out := make([]Group, 0, len(order))
	for _, kind := range order {
		items := grouped[kind]
		if len(items) == 0 {
			continue
		}
		out = append(out, Group{Kind: kind, Tunnels: items})
	}
	return out
}

// Format renders one aggregated notification.
func Format(cat Catalog, group Group) Notice {
	if len(group.Tunnels) == 0 {
		return Notice{Title: cat.Title}
	}
	if len(group.Tunnels) == 1 {
		tunnel := group.Tunnels[0]
		name := strings.TrimSpace(tunnel.Name)
		if name == "" {
			name = "#" + strconv.Itoa(tunnel.ID)
		}
		return Notice{
			Title:    cat.Title,
			Body:     fmt.Sprintf(oneTemplate(cat, group.Kind), name),
			TunnelID: tunnel.ID,
		}
	}
	count := len(group.Tunnels)
	noun := nounFor(cat, count)
	template := pluralTemplate(cat, group.Kind)
	if singularVerb(cat.Locale, count) {
		template = singularTemplate(cat, group.Kind)
	}
	return Notice{
		Title: cat.Title,
		Body:  fmt.Sprintf(template, count, noun),
	}
}

func oneTemplate(cat Catalog, kind Kind) string {
	switch kind {
	case KindDropped:
		return cat.DroppedOne
	case KindReconnected:
		return cat.ReconnectedOne
	case KindGaveUp:
		return cat.GaveUpOne
	case KindConnectFailed:
		return cat.ConnectFailedOne
	case KindConnected:
		return cat.ConnectedOne
	default:
		return "%s"
	}
}

func singularTemplate(cat Catalog, kind Kind) string {
	switch kind {
	case KindDropped:
		return cat.DroppedSingular
	case KindReconnected:
		return cat.ReconnectedSingular
	case KindGaveUp:
		return cat.GaveUpSingular
	case KindConnectFailed:
		return cat.ConnectFailedSingular
	case KindConnected:
		return cat.ConnectedSingular
	default:
		return "%d %s"
	}
}

func pluralTemplate(cat Catalog, kind Kind) string {
	switch kind {
	case KindDropped:
		return cat.DroppedPlural
	case KindReconnected:
		return cat.ReconnectedPlural
	case KindGaveUp:
		return cat.GaveUpPlural
	case KindConnectFailed:
		return cat.ConnectFailedPlural
	case KindConnected:
		return cat.ConnectedPlural
	default:
		return "%d %s"
	}
}

func nounFor(cat Catalog, count int) string {
	if cat.Locale == "en" {
		if count == 1 {
			return cat.TunnelOne
		}
		return cat.TunnelMany
	}
	switch pluralForm(count) {
	case "one":
		return cat.TunnelOne
	case "few":
		return cat.TunnelFew
	default:
		return cat.TunnelMany
	}
}

func singularVerb(locale string, count int) bool {
	if locale == "en" {
		return count == 1
	}
	return pluralForm(count) == "one"
}

func pluralForm(count int) string {
	n := count % 100
	if n < 0 {
		n = -n
	}
	if n >= 11 && n <= 14 {
		return "many"
	}
	switch n % 10 {
	case 1:
		return "one"
	case 2, 3, 4:
		return "few"
	default:
		return "many"
	}
}

// ParseFocusArg reads --norka-focus=<id> from process or toast arguments.
func ParseFocusArg(args []string) int {
	const prefix = "--norka-focus="
	for _, arg := range args {
		for _, field := range strings.Fields(arg) {
			if !strings.HasPrefix(field, prefix) {
				continue
			}
			n, err := strconv.Atoi(strings.TrimPrefix(field, prefix))
			if err != nil || n <= 0 {
				return 0
			}
			return n
		}
	}
	return 0
}
