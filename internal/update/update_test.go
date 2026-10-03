package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestCheck_PrefersLinuxAppImage(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.2.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.2.0",
		"assets": [
			{"name": "norka.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka.exe"},
			{"name": "norka_1.2.0_amd64.deb", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka_1.2.0_amd64.deb"},
			{"name": "other-x86_64.AppImage", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/other-x86_64.AppImage"},
			{"name": "norka-x86_64.AppImage", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka-x86_64.AppImage"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "linux", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available {
		t.Fatalf("available = false, offer = %+v", offer)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka-x86_64.AppImage" {
		t.Fatalf("url = %s", offer.URL)
	}
	if offer.CanApply {
		t.Fatal("linux AppImage must stay a link")
	}
}

func TestCheck_LinuxDebWhenNoAppImage(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.2.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.2.0",
		"assets": [
			{"name": "norka.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka.exe"},
			{"name": "norka_1.2.0_amd64.deb", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka_1.2.0_amd64.deb"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "linux", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/download/v1.2.0/norka_1.2.0_amd64.deb" {
		t.Fatalf("url = %s", offer.URL)
	}
	if offer.CanApply {
		t.Fatal("linux deb must stay a link")
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
		{"1.0.1", "1.0", true},
		{"v2", "1.9.9", true},
		{"1.0.0", "v1.0.0", false},
		{"1.2", "1.2.0", false},
		{"1.0", "1.0.1", false},
		{"v1.0.0-beta", "1.0.0", false},
		{"v1.2.0-rc.1", "1.0.0", false},
		{"v1.2.0+build", "1.2.0", false},
		{"nope", "1.0.0", false},
		{"", "1.0.0", false},
		{"1.0.0", "", false},
	}
	for _, tc := range cases {
		if got := versionNewer(tc.latest, tc.current); got != tc.want {
			t.Fatalf("versionNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestTrimNotes_StripsReleaseFooter(t *testing.T) {
	const marker = "<!-- norka:release-footer -->"
	footer := marker + "\n## Windows: неизвестный издатель\n\nПодробнее\n"
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "  hello  ", want: "hello"},
		{raw: "Заметки\n\n" + footer, want: "Заметки"},
		{raw: "Заметки  \n\n" + footer, want: "Заметки"},
		{raw: "\n\n" + footer, want: ""},
		{raw: "text " + marker + " rest", want: "text"},
		{raw: "a  b", want: "a  b"},
	}
	for _, tc := range cases {
		if got := trimNotes(tc.raw); got != tc.want {
			t.Fatalf("trimNotes(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestCheck_StripsReleaseFooter(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.4.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.4.0",
		"body": "Заметки\n\n<!-- norka:release-footer -->\n## Windows: неизвестный издатель\n",
		"assets": [
			{"name": "norka.dmg", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.4.0/norka.dmg"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.2", "darwin", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if offer.Notes != "Заметки" {
		t.Fatalf("notes = %q", offer.Notes)
	}
}

func TestCheck_IncludesNotesAndDigest(t *testing.T) {
	const sum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	srv := releaseServer(t, `{
		"tag_name": "v1.4.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.4.0",
		"body": "## Что нового\n\n- тише в трее",
		"assets": [
			{"name": "norka.exe", "size": 42, "digest": "sha256:`+sum+`", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.4.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.2", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available || !offer.CanApply {
		t.Fatalf("offer = %+v", offer)
	}
	if offer.Digest != sum || offer.AssetSize != 42 || offer.AssetName != "norka.exe" {
		t.Fatalf("asset = %+v", offer)
	}
	if offer.PageURL != "https://github.com/norka-app/Norka/releases/tag/v1.4.0" {
		t.Fatalf("page = %s", offer.PageURL)
	}
	if offer.Notes != "## Что нового\n\n- тише в трее" {
		t.Fatalf("notes = %q", offer.Notes)
	}
}

func TestCheck_PrefersExactAssetName(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.4.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.4.0",
		"assets": [
			{"name": "extra.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.4.0/extra.exe"},
			{"name": "norka.exe", "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.4.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/download/v1.4.0/norka.exe" {
		t.Fatalf("url = %s", offer.URL)
	}
}

func TestCheck_WithoutDigestCannotApply(t *testing.T) {
	srv := releaseServer(t, `{
		"tag_name": "v1.4.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.4.0",
		"assets": [
			{"name": "norka.exe", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.4.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available || offer.CanApply || offer.Digest != "" {
		t.Fatalf("expected browser fallback, got %+v", offer)
	}
}

func TestCheck_UsesChecksumFileWhenDigestMissing(t *testing.T) {
	const sum = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SHA256SUMS" {
			_, _ = w.Write([]byte(sum + "  norka.dmg\n"))
			return
		}
		fmt.Fprintf(w, `{
			"tag_name": "v3.0.0",
			"html_url": "https://github.com/norka-app/Norka/releases/tag/v3.0.0",
			"body": "notes",
			"assets": [
				{"name": "norka.dmg", "size": 9, "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v3.0.0/norka.dmg"},
				{"name": "SHA256SUMS", "browser_download_url": "%s/SHA256SUMS"}
			]
		}`, srv.URL)
	}))
	defer srv.Close()

	allow := func(raw string) bool {
		return acceptedURL(raw) || strings.HasPrefix(raw, srv.URL)
	}
	offer, err := checkPolicy(context.Background(), "1.0.0", "darwin", srv.URL, srv.Client(), allow)
	if err != nil {
		t.Fatal(err)
	}
	if !offer.CanApply || offer.Digest != sum || offer.AssetName != "norka.dmg" {
		t.Fatalf("offer = %+v", offer)
	}
}

func TestCheck_IgnoresPrereleaseAndDraft(t *testing.T) {
	for _, body := range []string{
		`{"tag_name":"v9.0.0","prerelease":true,"html_url":"https://github.com/norka-app/Norka/releases/tag/v9.0.0","assets":[{"name":"norka.exe","digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","browser_download_url":"https://github.com/norka-app/Norka/releases/download/v9.0.0/norka.exe"}]}`,
		`{"tag_name":"v9.0.0","draft":true,"html_url":"https://github.com/norka-app/Norka/releases/tag/v9.0.0","assets":[{"name":"norka.exe","digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","browser_download_url":"https://github.com/norka-app/Norka/releases/download/v9.0.0/norka.exe"}]}`,
		`{"tag_name":"v9.0.0-rc.1","html_url":"https://github.com/norka-app/Norka/releases/tag/v9.0.0-rc.1","assets":[{"name":"norka.exe","digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","browser_download_url":"https://github.com/norka-app/Norka/releases/download/v9.0.0-rc.1/norka.exe"}]}`,
	} {
		srv := releaseServer(t, body)
		offer, err := check(context.Background(), "1.0.0", "windows", srv.URL, srv.Client())
		srv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if offer.Available || offer.CanApply || offer.URL != "" {
			t.Fatalf("pre-release was offered: %+v body %s", offer, body)
		}
	}
}

func TestCheck_LinuxStaysALink(t *testing.T) {
	const sum = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	srv := releaseServer(t, `{
		"tag_name": "v1.5.0",
		"html_url": "https://github.com/norka-app/Norka/releases/tag/v1.5.0",
		"assets": [
			{"name": "norka.exe", "digest": "sha256:`+sum+`", "browser_download_url": "https://github.com/norka-app/Norka/releases/download/v1.5.0/norka.exe"}
		]
	}`)
	defer srv.Close()

	offer, err := check(context.Background(), "1.0.0", "linux", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Available || offer.CanApply {
		t.Fatalf("linux should be a plain link, got %+v", offer)
	}
	if offer.URL != "https://github.com/norka-app/Norka/releases/tag/v1.5.0" {
		t.Fatalf("url = %s", offer.URL)
	}
}

func TestAllowedRedirect(t *testing.T) {
	cases := []struct {
		raw string
		ok  bool
	}{
		{"https://release-assets.githubusercontent.com/abc", true},
		{"https://objects.githubusercontent.com/github-production-release-asset/1", true},
		{"https://github-releases.githubusercontent.com/1", true},
		{"https://github.com/norka-app/Norka/releases/download/v1/norka.exe", true},
		{"https://api.github.com/repos/norka-app/Norka/releases/latest", true},
		{"https://github.com/other/Norka/releases/download/v1/norka.exe", false},
		{"https://evil.example/norka.exe", false},
		{"http://release-assets.githubusercontent.com/abc", false},
		{"https://api.github.com/repos/other/Norka/releases/latest", false},
	}
	for _, tc := range cases {
		parsed, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := allowedRedirect(parsed); got != tc.ok {
			t.Fatalf("allowedRedirect(%s) = %v, want %v", tc.raw, got, tc.ok)
		}
	}
}

func TestSuppressSkipped(t *testing.T) {
	offer := Offer{Available: true, Latest: "1.2.0", URL: "https://example", CanApply: true, Notes: "hi"}
	hidden := SuppressSkipped(offer, "1.2.0", false)
	if hidden.Available || hidden.URL != "" || hidden.CanApply {
		t.Fatalf("skipped offer still visible: %+v", hidden)
	}
	shown := SuppressSkipped(offer, "1.2.0", true)
	if !shown.Available || !shown.CanApply {
		t.Fatalf("manual check should ignore skip: %+v", shown)
	}
	other := SuppressSkipped(offer, "1.1.0", false)
	if !other.Available {
		t.Fatalf("older skip should not hide 1.2.0")
	}
}

func releaseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}
