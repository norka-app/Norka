package biz

import (
	"errors"
	"strings"

	"github.com/norka-app/Norka/internal/localbind"
	"github.com/norka-app/Norka/internal/model"
)

// startFailureReason keeps the OS listen error, and prefixes a translatable
// hint when the local port is busy and no other Norka tunnel holds it.
func (b *TunnelBiz) startFailureReason(tunnel model.Tunnel, err error) string {
	raw := errReason(err)
	items, listErr := b.List()
	if listErr != nil {
		return raw
	}
	return externalBindReason(err, tunnel, items)
}

func externalBindReason(err error, tunnel model.Tunnel, siblings []model.Tunnel) string {
	raw := errReason(err)
	var busy *localbind.Error
	if !errors.As(err, &busy) {
		return raw
	}
	mode := strings.TrimSpace(tunnel.Mode)
	if mode != "" && mode != "local" {
		return raw
	}
	if len(PortConflicts(tunnel, siblings)) > 0 {
		return raw
	}
	remote := tunnel.RemotePort
	if remote < 0 {
		remote = 0
	}
	suggested := localbind.SuggestPort(tunnel.LocalHost, tunnel.LocalPort)
	return localbind.Format(tunnel.LocalPort, suggested, remote, raw)
}
