package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Apply downloads the offered asset, checks its size and sha256, and replaces
// the running app when the install location is writable. Otherwise it asks the
// UI to open the download in the browser.
func Apply(ctx context.Context, offer Offer, progress func(Progress)) (ApplyResult, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	if !offer.Available || strings.TrimSpace(offer.URL) == "" {
		return ApplyResult{Code: CodeNone}, nil
	}
	if !offer.CanApply {
		return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeUnsupported}, nil
	}
	if devMode() || !installTargetWritable() {
		return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeNotWritable}, nil
	}

	dest, err := stagePath(offer.AssetName)
	if err != nil {
		return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeDownload}, nil
	}
	keepDownload := false
	defer func() {
		if !keepDownload {
			_ = os.Remove(dest)
		}
	}()

	err = downloadVerified(ctx, downloadSpec{
		URL:    offer.URL,
		Dest:   dest,
		Size:   offer.AssetSize,
		SHA256: offer.Digest,
		Progress: func(p Progress) {
			progress(p)
		},
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ApplyResult{Code: CodeCancelled}, nil
		}
		if errors.Is(err, errChecksum) {
			return ApplyResult{URL: offer.URL, Code: CodeChecksum}, nil
		}
		if errors.Is(err, errSize) {
			return ApplyResult{URL: offer.URL, Code: CodeSize}, nil
		}
		return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeDownload}, nil
	}

	progress(Progress{Phase: PhaseVerify, Received: offer.AssetSize, Total: offer.AssetSize, Percent: 100})
	if err := verifyFile(dest, offer.AssetSize, offer.Digest); err != nil {
		if errors.Is(err, errSize) {
			return ApplyResult{URL: offer.URL, Code: CodeSize}, nil
		}
		return ApplyResult{URL: offer.URL, Code: CodeChecksum}, nil
	}

	progress(Progress{Phase: PhaseInstall, Percent: 100, Total: offer.AssetSize, Received: offer.AssetSize})
	if err := installPrepared(ctx, dest); err != nil {
		if errors.Is(err, errNotWritable) {
			return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeNotWritable}, nil
		}
		if runtime.GOOS == "darwin" {
			if openErr := openLocalFile(dest); openErr == nil {
				keepDownload = true
				return ApplyResult{Fallback: true, Code: CodeInstall}, nil
			}
		}
		return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeInstall}, nil
	}
	if err := scheduleRelaunch(); err != nil {
		return ApplyResult{Fallback: true, URL: offer.URL, Code: CodeInstall}, nil
	}
	progress(Progress{Phase: PhaseRestart, Percent: 100})
	return ApplyResult{Restarting: true}, nil
}

func stagePath(assetName string) (string, error) {
	name := filepath.Base(strings.TrimSpace(assetName))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "", fmt.Errorf("asset name is empty")
	}
	dir := filepath.Join(os.TempDir(), "norka-update")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func devMode() bool {
	if strings.TrimSpace(os.Getenv("devserver")) != "" {
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(filepath.ToSlash(exe), "/go-build")
}

func installTargetWritable() bool {
	dir, ok := installDir()
	if !ok {
		return false
	}
	return dirWritable(dir)
}

func dirWritable(dir string) bool {
	file, err := os.CreateTemp(dir, ".norka-write-*")
	if err != nil {
		return false
	}
	name := file.Name()
	file.Close()
	return os.Remove(name) == nil
}

// swapFile renames current to current.old and copies src into its place.
func swapFile(current, src string) error {
	backup := current + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(current, backup); err != nil {
		return err
	}
	if err := copyFile(src, current, 0o755); err != nil {
		_ = os.Remove(current)
		_ = os.Rename(backup, current)
		return err
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(dst, mode)
}

// CleanupBackup removes the previous executable or app bundle left behind by
// the last successful update. The file may still be locked for a moment.
func CleanupBackup() {
	target, ok := backupPath()
	if !ok {
		return
	}
	go removeWithRetry(target)
}

func removeWithRetry(path string) {
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(path); err != nil {
			return
		}
		if os.RemoveAll(path) == nil {
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
}

func backupPath() (string, bool) {
	if runtime.GOOS == "darwin" {
		if bundle, ok := runningBundlePath(); ok {
			return bundle + ".old", true
		}
		return "", false
	}
	if runtime.GOOS != "windows" {
		return "", false
	}
	exe, err := currentExecutable()
	if err != nil {
		return "", false
	}
	return exe + ".old", true
}

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

func runningBundlePath() (string, bool) {
	exe, err := currentExecutable()
	if err != nil {
		return "", false
	}
	return bundleFromExe(exe)
}

func bundleFromExe(exe string) (string, bool) {
	dir := filepath.Dir(exe)
	if !strings.EqualFold(filepath.Base(dir), "MacOS") {
		return "", false
	}
	contents := filepath.Dir(dir)
	if filepath.Base(contents) != "Contents" {
		return "", false
	}
	bundle := filepath.Dir(contents)
	if !strings.HasSuffix(strings.ToLower(filepath.Base(bundle)), ".app") {
		return "", false
	}
	return bundle, true
}

func installDir() (string, bool) {
	switch runtime.GOOS {
	case "windows":
		exe, err := currentExecutable()
		if err != nil {
			return "", false
		}
		return filepath.Dir(exe), true
	case "darwin":
		bundle, ok := runningBundlePath()
		if !ok {
			return "", false
		}
		return filepath.Dir(bundle), true
	default:
		return "", false
	}
}

var errNotWritable = errors.New("install location is not writable")
