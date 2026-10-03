package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/norka-app/Norka/internal/ipc"
)

func usage(locale string) string {
	if strings.HasPrefix(locale, "ru") {
		return strings.TrimSpace(`
norka connect <имя|id>      подключить туннель
norka disconnect <имя|id>   отключить туннель
norka toggle <имя|id>       переключить туннель
norka status [--json]       состояние Norka и туннелей
norka list [--json]         список туннелей

Коды выхода:
  0  успех
  1  автоматизация выключена (Настройки → Функции)
  2  Norka не запущена
  3  туннель не найден
  4  имя неоднозначно
  5  неверные аргументы
  6  нет связи с локальным каналом
  7  команда туннеля не выполнена
`)
	}
	return strings.TrimSpace(`
norka connect <name|id>      connect a tunnel
norka disconnect <name|id>   disconnect a tunnel
norka toggle <name|id>       toggle a tunnel
norka status [--json]        Norka and tunnel status
norka list [--json]          list tunnels

Exit codes:
  0  success
  1  automation is off (Settings → Features)
  2  Norka is not running
  3  tunnel was not found
  4  the name is ambiguous
  5  invalid arguments
  6  the local channel could not be reached
  7  the tunnel command failed
`)
}

func writeHuman(w io.Writer, resp ipc.Response) {
	if strings.TrimSpace(resp.Message) != "" {
		fmt.Fprintln(w, resp.Message)
	}
	if len(resp.Tunnels) == 0 {
		return
	}
	if resp.Message != "" {
		fmt.Fprintln(w)
	}
	for _, tunnel := range resp.Tunnels {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", tunnel.ID, tunnel.Name, tunnel.Status, tunnel.Mode, addressOf(tunnel))
	}
}

func addressOf(tunnel ipc.TunnelInfo) string {
	host := tunnel.LocalHost
	if host == "" {
		host = "127.0.0.1"
	}
	if tunnel.LocalPort <= 0 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, tunnel.LocalPort)
}

func writeJSON(w io.Writer, resp ipc.Response) error {
	data, err := jsonMarshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
