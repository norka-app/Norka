package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoWailsOrSystrayDeps(t *testing.T) {
	out := goCmd(t, "list", "-deps", "./internal/engine/...")
	for _, pkg := range strings.Fields(out) {
		if strings.Contains(pkg, "github.com/wailsapp") || strings.Contains(pkg, "systray") {
			t.Errorf("engine depends on a GUI package: %s", pkg)
		}
	}
}

func TestCGODisabledBuild(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			cmd := exec.Command("go", "build", "./internal/engine/...")
			cmd.Dir = moduleRoot(t)
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH=amd64")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("CGO_ENABLED=0 GOOS=%s go build ./internal/engine/...: %v\n%s", goos, err, out)
			}
		})
	}
}

func goCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
