package automation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/norka-app/Norka/internal/model"
)

// AmbiguousError means more than one tunnel matched the query.
type AmbiguousError struct {
	Query   string
	Matches []model.Tunnel
}

func (e *AmbiguousError) Error() string {
	names := make([]string, 0, len(e.Matches))
	for _, tunnel := range e.Matches {
		names = append(names, tunnel.Name)
	}
	return fmt.Sprintf("ambiguous tunnel %q: %s", e.Query, strings.Join(names, ", "))
}

// Names returns the matched tunnel names in catalog order.
func (e *AmbiguousError) Names() []string {
	if e == nil {
		return nil
	}
	names := make([]string, 0, len(e.Matches))
	for _, tunnel := range e.Matches {
		names = append(names, tunnel.Name)
	}
	return names
}

// Match finds a tunnel by exact name (case-insensitive), then by id, then by
// a unique name prefix. An exact name that hits more than one tunnel is
// ambiguous and does not fall through to id or prefix.
func Match(tunnels []model.Tunnel, query string) (model.Tunnel, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return model.Tunnel{}, fmt.Errorf("tunnel name is empty")
	}

	exact := make([]model.Tunnel, 0, 1)
	for _, tunnel := range tunnels {
		if strings.EqualFold(tunnel.Name, query) {
			exact = append(exact, tunnel)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return model.Tunnel{}, &AmbiguousError{Query: query, Matches: exact}
	}

	if id, ok := canonicalID(query); ok {
		for _, tunnel := range tunnels {
			if tunnel.ID == id {
				return tunnel, nil
			}
		}
	}

	folded := strings.ToLower(query)
	prefix := make([]model.Tunnel, 0, 1)
	for _, tunnel := range tunnels {
		if strings.HasPrefix(strings.ToLower(tunnel.Name), folded) {
			prefix = append(prefix, tunnel)
		}
	}
	if len(prefix) == 1 {
		return prefix[0], nil
	}
	if len(prefix) > 1 {
		return model.Tunnel{}, &AmbiguousError{Query: query, Matches: prefix}
	}
	return model.Tunnel{}, fmt.Errorf("tunnel %q not found", query)
}

// canonicalID accepts a positive base-10 id with no leading zeros or sign.
func canonicalID(query string) (int, bool) {
	if query == "" || query[0] < '1' || query[0] > '9' {
		return 0, false
	}
	for _, r := range query[1:] {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(query)
	if err != nil || id <= 0 || strconv.Itoa(id) != query {
		return 0, false
	}
	return id, true
}
