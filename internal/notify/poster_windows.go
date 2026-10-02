//go:build windows

package notify

import (
	"log/slog"
	"os"
	"strings"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

type windowsPoster struct {
	appID string
	icon  string
}

func newPlatformPoster(cfg PosterConfig) Poster {
	setClickHandler(cfg.OnClick)
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("toast activation executable unavailable", "error", err)
		exe = ""
	} else if strings.Contains(exe, " ") {
		exe = `"` + exe + `"`
	}
	if err := toast.SetAppData(toast.AppData{
		AppID:         cfg.AppID,
		IconPath:      cfg.IconPath,
		ActivationExe: exe,
	}); err != nil {
		slog.Warn("toast app identity was not registered", "error", err)
	}
	toast.SetActivationCallback(func(args string, _ []toast.UserData) {
		dispatchClick(ParseFocusArg([]string{args}))
	})
	return &windowsPoster{appID: cfg.AppID, icon: cfg.IconPath}
}

func (p *windowsPoster) Post(notice Notice) error {
	notification := toast.Notification{
		AppID:               p.appID,
		Title:               notice.Title,
		Body:                notice.Body,
		Icon:                p.icon,
		ActivationType:      toast.Foreground,
		ActivationArguments: focusArg(notice.TunnelID),
		Audio:               toast.Default,
	}
	return notification.Push()
}
