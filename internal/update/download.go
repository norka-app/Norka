package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type downloadSpec struct {
	URL           string
	Dest          string
	Size          int64
	SHA256        string
	Allow         func(string) bool
	AllowRedirect func(*url.URL) bool
	Progress      func(Progress)
	Client        *http.Client
}

func downloadVerified(ctx context.Context, spec downloadSpec) error {
	allow := spec.Allow
	if allow == nil {
		allow = acceptedURL
	}
	rawURL := strings.TrimSpace(spec.URL)
	if !allow(rawURL) {
		return fmt.Errorf("download url refused")
	}
	expected := strings.ToLower(strings.TrimSpace(spec.SHA256))
	if !isSHA256Hex(expected) {
		return errChecksum
	}
	if err := os.MkdirAll(dirOf(spec.Dest), 0o700); err != nil {
		return err
	}

	allowRedirect := spec.AllowRedirect
	if allowRedirect == nil {
		allowRedirect = allowedRedirect
	}
	client := spec.Client
	if client == nil {
		client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("too many redirects")
				}
				if !allowRedirect(req.URL) {
					return fmt.Errorf("redirect refused")
				}
				return nil
			},
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Norka")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	if spec.Size > 0 && resp.ContentLength > 0 && resp.ContentLength != spec.Size {
		return errSize
	}

	tmp := spec.Dest + ".part"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		file.Close()
		if _, statErr := os.Stat(spec.Dest); statErr != nil {
			_ = os.Remove(tmp)
		}
	}()

	hasher := sha256.New()
	buf := make([]byte, 32*1024)
	var received int64
	total := spec.Size
	if total <= 0 && resp.ContentLength > 0 {
		total = resp.ContentLength
	}
	lastReport := time.Time{}
	report := func(force bool) {
		if spec.Progress == nil {
			return
		}
		if !force && time.Since(lastReport) < 80*time.Millisecond {
			return
		}
		lastReport = time.Now()
		spec.Progress(Progress{
			Phase:    PhaseDownload,
			Received: received,
			Total:    total,
			Percent:  percentOf(received, total),
		})
	}

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return err
			}
			if _, err := hasher.Write(buf[:n]); err != nil {
				return err
			}
			received += int64(n)
			report(false)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	report(true)
	if err := file.Close(); err != nil {
		return err
	}
	if spec.Size > 0 && received != spec.Size {
		return errSize
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if got != expected {
		return errChecksum
	}
	if err := os.Rename(tmp, spec.Dest); err != nil {
		return err
	}
	return nil
}

func percentOf(received, total int64) int {
	if total <= 0 {
		return 0
	}
	if received >= total {
		return 100
	}
	return int(received * 100 / total)
}

func dirOf(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return filepath.Dir(path)
}
