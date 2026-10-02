package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadVerifiedOK(t *testing.T) {
	payload := []byte("norka-binary")
	sum := sha256Hex(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing user agent")
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "norka.exe")
	var last Progress
	err := downloadVerified(context.Background(), downloadSpec{
		URL:           srv.URL,
		Dest:          dest,
		Size:          int64(len(payload)),
		SHA256:        sum,
		Allow:         func(string) bool { return true },
		AllowRedirect: func(*url.URL) bool { return false },
		Progress:      func(p Progress) { last = p },
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("body = %q", got)
	}
	if last.Phase != PhaseDownload || last.Percent != 100 {
		t.Fatalf("progress = %+v", last)
	}
}

func TestDownloadVerifiedRejectsBadHashAndSize(t *testing.T) {
	payload := []byte("norka-binary")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "norka.exe")
	err := downloadVerified(context.Background(), downloadSpec{
		URL:    srv.URL,
		Dest:   dest,
		Size:   int64(len(payload)),
		SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Allow:  func(string) bool { return true },
	})
	if err != errChecksum {
		t.Fatalf("hash err = %v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatal("failed download should not leave the destination")
	}

	err = downloadVerified(context.Background(), downloadSpec{
		URL:    srv.URL,
		Dest:   dest,
		Size:   int64(len(payload) + 3),
		SHA256: sha256Hex(payload),
		Allow:  func(string) bool { return true },
	})
	if err != errSize {
		t.Fatalf("size err = %v", err)
	}
}

func TestDownloadVerifiedRejectsUntrustedURLAndRedirect(t *testing.T) {
	payload := []byte("ok")
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pwned"))
	}))
	defer evil.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/norka.exe", http.StatusFound)
	}))
	defer good.Close()

	dest := filepath.Join(t.TempDir(), "norka.exe")
	err := downloadVerified(context.Background(), downloadSpec{
		URL:    good.URL,
		Dest:   dest,
		SHA256: sha256Hex(payload),
		Allow:  func(string) bool { return false },
	})
	if err == nil {
		t.Fatal("expected untrusted url to fail")
	}

	err = downloadVerified(context.Background(), downloadSpec{
		URL:    good.URL,
		Dest:   dest,
		Size:   int64(len(payload)),
		SHA256: sha256Hex(payload),
		Allow:  func(string) bool { return true },
		AllowRedirect: func(u *url.URL) bool {
			return allowedRedirect(u)
		},
	})
	if err == nil {
		t.Fatal("expected foreign redirect to fail")
	}

	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer mirror.Close()
	hopper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, mirror.URL+"/file", http.StatusFound)
	}))
	defer hopper.Close()
	mirrorURL, _ := url.Parse(mirror.URL)
	err = downloadVerified(context.Background(), downloadSpec{
		URL:    hopper.URL,
		Dest:   dest,
		Size:   int64(len(payload)),
		SHA256: sha256Hex(payload),
		Allow:  func(string) bool { return true },
		AllowRedirect: func(u *url.URL) bool {
			return u.Host == mirrorURL.Host
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
