package notify

import (
	"log/slog"
	"os/exec"
	"strconv"
	"sync"
)

// PosterConfig is the OS notification identity.
type PosterConfig struct {
	AppID    string
	IconPath string
	OnClick  func(tunnelID int)
}

// NewSystemPoster returns the native notifier for this OS.
func NewSystemPoster(cfg PosterConfig) Poster {
	if cfg.AppID == "" {
		cfg.AppID = "Norka"
	}
	return newPlatformPoster(cfg)
}

type commandPoster struct {
	appID   string
	onClick func(int)
}

func (p *commandPoster) Post(notice Notice) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		slog.Info("notification not shown; notify-send is unavailable", "title", notice.Title, "body", notice.Body)
		return nil
	}
	cmd := exec.Command("notify-send", "-a", p.appID, notice.Title, notice.Body)
	return cmd.Run()
}

var (
	clickMu sync.Mutex
	clickFn func(int)
)

func setClickHandler(fn func(int)) {
	clickMu.Lock()
	clickFn = fn
	clickMu.Unlock()
}

func dispatchClick(tunnelID int) {
	clickMu.Lock()
	fn := clickFn
	clickMu.Unlock()
	if fn == nil {
		return
	}
	go fn(tunnelID)
}

func focusArg(tunnelID int) string {
	if tunnelID <= 0 {
		return "--norka-focus=0"
	}
	return "--norka-focus=" + strconv.Itoa(tunnelID)
}
