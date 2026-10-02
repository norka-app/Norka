//go:build !windows && !darwin && !linux

package notify

func newPlatformPoster(cfg PosterConfig) Poster {
	return &commandPoster{appID: cfg.AppID, onClick: cfg.OnClick}
}
