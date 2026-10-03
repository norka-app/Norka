package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/norka-app/Norka/internal/conf"
)

const logFileName = "norkad.log"

func setupLog(configPath string, foreground bool) error {
	level := logLevel()
	if foreground {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		slog.Info("norkad logging to stderr")
		return nil
	}
	dir := filepath.Dir(strings.TrimSpace(configPath))
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, conf.PrivateDirPerm); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	logPath := filepath.Join(dir, logFileName)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, conf.PrivateFilePerm)
	if err != nil {
		return fmt.Errorf("open %s: %w", logPath, err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: level})))
	slog.Info("norkad logging", "path", logPath)
	return nil
}

func logLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NORKA_LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func logReady(configPath string) {
	slog.Info("norkad ready", "config", configPath, "pid", os.Getpid())
}

func logStopping() {
	slog.Info("norkad shutting down")
}

func logStopped() {
	slog.Info("norkad stopped")
}
