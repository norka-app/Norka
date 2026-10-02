package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheck_OffersNewerWindowsExe(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.2.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.2.0",
		"assets": [
			{"name": "notes.txt", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/notes.txt"},
			{"name": "norka.dmg", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka.dmg"},
			{"name": "norka.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available {
		t.Fatalf("available = false, offer = %+v", offer)
	}
	if offer.Latest != "1.2.0" || offer.Current != "1.0.0" {
		t.Fatalf("versions = %s -> %s", offer.Current, offer.Latest)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka.exe" {
		t.Fatalf("url = %s", offer.URL)
	}
}

func TestCheck_OffersNewerMacDMG(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.2.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.2.0",
		"assets": [
			{"name": "extra.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/extra.exe"},
			{"name": "Norka.dmg", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/Norka.dmg"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "v1.1.9", "darwin", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available {
		t.Fatalf("available = false, offer = %+v", offer)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/download/v1.2.0/Norka.dmg" {
		t.Fatalf("url = %s", offer.URL)
	}
}

func TestCheck_SameVersionIsNotAnUpdate(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.0.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.0.0",
		"assets": [
			{"name": "norka.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.0.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if offer.Available || offer.URL != "" {
		t.Fatalf("same version should not be offered, got %+v", offer)
	}
}

func TestCheck_OlderReleaseIsNotAnUpdate(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v0.9.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v0.9.0",
		"assets": [
			{"name": "norka.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v0.9.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if offer.Available {
		t.Fatalf("older release was offered: %+v", offer)
	}
}

func TestCheck_RejectsForeignDownloadURL(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v2.0.0",
		"html_url": "https://example.com/norka",
		"assets": [
			{"name": "norka.exe", "browser_download_url": "https://evil.example/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if offer.Available || offer.URL != "" {
		t.Fatalf("foreign url should not be offered, got %+v", offer)
	}
}

func TestCheck_FallsBackToReleasePage(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.3.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.3.0",
		"assets": []
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "linux", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available {
		t.Fatalf("release page should be offered, got %+v", offer)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/tag/v1.3.0" {
		t.Fatalf("url = %s", offer.URL)
	}
}

func TestCheck_GitHubError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing user agent")
		}
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v1.2.0", "1.0.0", true},
		{"v1.10.0", "1.9.0", true},
		{"1.0.0", "v1.0.0", false},
		{"v1.0.0-beta", "1.0.0", false},
		{"nope", "1.0.0", false},
	}
	for _, tc := range cases {
		if got := versionNewer(tc.latest, tc.current); got != tc.want {
			t.Fatalf("versionNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func releaseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}
