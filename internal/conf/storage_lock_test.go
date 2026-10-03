package conf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/filelock"
	"github.com/norka-app/Norka/internal/model"
)

func TestMain(m *testing.M) {
	switch os.Getenv("NORKA_CONF_HELPER") {
	case "update":
		confUpdateHelper()
		os.Exit(0)
	case "hold":
		confHoldHelper()
		os.Exit(0)
	default:
		os.Exit(m.Run())
	}
}

func TestStorageConcurrentUpdate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	const perProcess = 30
	ctxTimeout := 30 * time.Second

	type child struct {
		cmd *exec.Cmd
		out *bytes.Buffer
	}
	var children []child
	for _, id := range []string{"a", "b"} {
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(),
			"NORKA_CONF_HELPER=update",
			"NORKA_CONF_PATH="+configPath,
			"NORKA_CONF_ID="+id,
			"NORKA_CONF_N="+strconv.Itoa(perProcess),
		)
		cmd.Dir = dir
		buf := &bytes.Buffer{}
		cmd.Stdout = buf
		cmd.Stderr = buf
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, child{cmd: cmd, out: buf})
	}
	t.Cleanup(func() {
		for _, ch := range children {
			if ch.cmd.Process != nil {
				_ = ch.cmd.Process.Kill()
			}
		}
	})

	deadline := time.Now().Add(10 * time.Second)
	for _, id := range []string{"a", "b"} {
		ready := filepath.Join(dir, "ready-"+id)
		for {
			if _, err := os.Stat(ready); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("process %s did not reach the start barrier", id)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte("go\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	errc := make(chan error, len(children))
	for _, ch := range children {
		ch := ch
		go func() {
			timer := time.AfterFunc(ctxTimeout, func() {
				if ch.cmd.Process != nil {
					_ = ch.cmd.Process.Kill()
				}
			})
			defer timer.Stop()
			err := ch.cmd.Wait()
			if err != nil {
				errc <- fmt.Errorf("%v\n%s", err, ch.out.String())
				return
			}
			errc <- nil
		}()
	}
	for range children {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}

	storage, err := NewStorage(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, jumper := range cfg.Jumpers {
		seen[jumper.Name] = struct{}{}
	}
	if len(cfg.Jumpers) != perProcess*2 || len(seen) != perProcess*2 {
		names := make([]string, 0, len(cfg.Jumpers))
		for _, jumper := range cfg.Jumpers {
			names = append(names, jumper.Name)
		}
		t.Fatalf("got %d jumpers (%d unique), want %d\n%v", len(cfg.Jumpers), len(seen), perProcess*2, names)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, ".norka-config-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

func TestStorageLockTimeout(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	storage, err := NewStorage(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Load(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"NORKA_CONF_HELPER=hold",
		"NORKA_CONF_PATH="+configPath,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
	if err := waitReady(stdout); err != nil {
		t.Fatal(err)
	}

	previous := configLockTimeout
	configLockTimeout = 300 * time.Millisecond
	t.Cleanup(func() { configLockTimeout = previous })

	_, err = storage.Update(func(cfg *Config) error {
		cfg.AutoRun = true
		return nil
	})
	if !errors.Is(err, filelock.ErrTimeout) {
		t.Fatalf("Update error = %v, want timeout", err)
	}
	if !strings.Contains(err.Error(), "config.toml.lock") {
		t.Fatalf("timeout error does not name the lock file: %v", err)
	}
}

func confUpdateHelper() {
	path := os.Getenv("NORKA_CONF_PATH")
	id := os.Getenv("NORKA_CONF_ID")
	n, err := strconv.Atoi(os.Getenv("NORKA_CONF_N"))
	if err != nil || n <= 0 || path == "" || id == "" {
		fmt.Fprintf(os.Stderr, "bad helper env path=%q id=%q n=%q\n", path, id, os.Getenv("NORKA_CONF_N"))
		os.Exit(1)
	}
	dir := filepath.Dir(path)
	if err := os.WriteFile(filepath.Join(dir, "ready-"+id), []byte("ok\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	goFile := filepath.Join(dir, "go")
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(goFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "timeout waiting to start")
			os.Exit(1)
		}
		time.Sleep(5 * time.Millisecond)
	}

	storage, err := NewStorage(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("%s-%d", id, i)
		_, err := storage.Update(func(cfg *Config) error {
			next := 1
			for _, jumper := range cfg.Jumpers {
				if jumper.ID >= next {
					next = jumper.ID + 1
				}
			}
			cfg.Jumpers = append(cfg.Jumpers, model.Jumper{
				ID:   next,
				Name: name,
				Host: "127.0.0.1",
			})
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func confHoldHelper() {
	path := os.Getenv("NORKA_CONF_PATH")
	if path == "" {
		fmt.Fprintln(os.Stderr, "config path is empty")
		os.Exit(1)
	}
	lk, err := filelock.Acquire(path+".lock", 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer lk.Release()
	fmt.Println("ready")
	_ = os.Stdout.Sync()
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func waitReady(r io.Reader) error {
	buf := make([]byte, 16)
	done := make(chan error, 1)
	go func() {
		n, err := r.Read(buf)
		if err != nil {
			done <- err
			return
		}
		if !strings.HasPrefix(string(buf[:n]), "ready") {
			done <- fmt.Errorf("child said %q", buf[:n])
			return
		}
		done <- nil
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New("timeout waiting for lock holder")
	}
}
