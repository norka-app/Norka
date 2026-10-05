package localbind

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFrontendLocalBindHint(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	script := filepath.Join(filepath.Dir(file), "..", "..", "frontend", "src", "utils", "local-bind-error.test.js")
	cmd := exec.Command("node", "--test", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
