package releaseguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckReleaseTag(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	git("init", "-b", "main")
	write("wails.json", "{\n  \"info\": {\"productVersion\": \"1.7.0\"}\n}\n")
	write("docs/releases/v1.7.0.md", "## Русский\n\nТекст выпуска.\n")
	git("add", ".")
	git("commit", "-m", "release")
	mainSHA := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))

	if out, err := runCheck(t, root, "--tag", "v1.7.0", "--commit", mainSHA, "--main", "main"); err != nil {
		t.Fatalf("тег на main: %v\n%s", err, out)
	}

	git("checkout", "-b", "stale")
	write("wails.json", "{\n  \"info\": {\"productVersion\": \"1.1.0\"}\n}\n")
	if err := os.Remove(filepath.Join(root, "docs/releases/v1.7.0.md")); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "stale")
	staleSHA := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))

	out, err := runCheck(t, root, "--tag", "v1.7.0", "--commit", staleSHA, "--main", "main")
	if err == nil {
		t.Fatalf("устаревший коммит прошёл проверку:\n%s", out)
	}
	text := string(out)
	for _, want := range []string{"не входит", "1.1.0", "docs/releases/v1.7.0.md"} {
		if !strings.Contains(text, want) {
			t.Fatalf("в выводе нет %q:\n%s", want, text)
		}
	}

	git("checkout", "main")
	write("wails.json", "{\n  \"info\": {\"productVersion\": \"1.6.1\"}\n}\n")
	git("add", ".")
	git("commit", "-m", "wrong version")
	wrongSHA := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	out, err = runCheck(t, root, "--tag", "v1.7.0", "--commit", wrongSHA, "--main", "main")
	if err == nil {
		t.Fatal("несовпадение версии прошло проверку")
	}
	if !strings.Contains(string(out), "1.6.1") || strings.Contains(string(out), "не входит") {
		t.Fatalf("ожидалась только ошибка версии:\n%s", out)
	}
}

func TestWindowsUpdateRelaunchHidesConsole(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	repo := filepath.Join(filepath.Dir(file), "..", "..")
	apply, err := os.ReadFile(filepath.Join(repo, "internal/update/apply_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(apply)
	if strings.Contains(text, `exec.Command("cmd"`) || strings.Contains(text, "windowsRelaunchScript") {
		t.Fatal("перезапуск на Windows снова идёт через cmd")
	}
	if !strings.Contains(text, "HideWindow:") || !strings.Contains(text, "CREATE_NO_WINDOW") {
		t.Fatal("у перезапуска нет HideWindow и CREATE_NO_WINDOW")
	}

	execRun, err := os.ReadFile(filepath.Join(repo, "exec_run_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(execRun)
	if !strings.Contains(body, "HideWindow:") || !strings.Contains(body, "CREATE_NO_WINDOW") {
		t.Fatal("execRun на Windows не прячет консоль")
	}
}

func runCheck(t *testing.T, root string, args ...string) ([]byte, error) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	script := filepath.Join(filepath.Dir(file), "..", "check-release-tag.sh")
	cmd := exec.Command(script, append([]string{"--root", root}, args...)...)
	return cmd.CombinedOutput()
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}
