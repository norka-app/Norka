package netwatch

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

// interfacePollInterval is the fallback when the OS has no network notification.
const interfacePollInterval = 5 * time.Second

func watchInterfaces(ctx context.Context, out chan<- Event) {
	prev, err := interfaceSignature()
	if err != nil {
		prev = ""
	}
	ticker := time.NewTicker(interfacePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next, err := interfaceSignature()
			if err != nil {
				continue
			}
			if prev != "" && next != prev {
				emit(ctx, out, Event{Kind: KindNetwork})
			}
			prev = next
		}
	}
}

func interfaceSignature() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Index < ifaces[j].Index })
	var b strings.Builder
	for _, iface := range ifaces {
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			addrs = nil
		}
		parts := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			if addr == nil {
				continue
			}
			parts = append(parts, addr.String())
		}
		sort.Strings(parts)
		fmt.Fprintf(&b, "%d|%s|%s|%s|%s\n", iface.Index, iface.Name, iface.Flags.String(), iface.HardwareAddr.String(), strings.Join(parts, ","))
	}
	return b.String(), nil
}
