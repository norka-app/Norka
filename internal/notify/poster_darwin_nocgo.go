//go:build darwin && !cgo

package notify

import "log/slog"

// Cross-compiles and other cgo-off builds cannot link UserNotifications.
// The packaged macOS app uses the cgo poster instead.
type noopPoster struct{}

func newPlatformPoster(cfg PosterConfig) Poster {
	return noopPoster{}
}

func (noopPoster) Post(notice Notice) error {
	slog.Info("notification not shown; native poster needs cgo", "title", notice.Title)
	return nil
}
