package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	releaseURL = "https://api.github.com/repos/norka-app/Norka/releases/latest"
	repoPrefix = "/norka-app/Norka/"
	notesLimit = 20_000
)

// Offer is the result of comparing the running app with the latest GitHub release.
type Offer struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
	PageURL   string `json:"pageUrl"`
	Notes     string `json:"notes"`
	AssetName string `json:"assetName"`
	AssetSize int64  `json:"assetSize"`
	Digest    string `json:"digest"`
	CanApply  bool   `json:"canApply"`
}

// Progress is emitted while an update is downloaded and installed.
type Progress struct {
	Phase    string `json:"phase"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Percent  int    `json:"percent"`
}

// ApplyResult tells the UI whether the app is restarting or the user should
// download the file another way.
type ApplyResult struct {
	Restarting bool   `json:"restarting"`
	Fallback   bool   `json:"fallback"`
	URL        string `json:"url"`
	Code       string `json:"code,omitempty"`
}

const (
	PhaseDownload = "download"
	PhaseVerify   = "verify"
	PhaseInstall  = "install"
	PhaseRestart  = "restart"

	CodeNone        = "none"
	CodeUnsupported = "unsupported"
	CodeNotWritable = "not_writable"
	CodeDownload    = "download"
	CodeChecksum    = "checksum"
	CodeSize        = "size"
	CodeInstall     = "install"
	CodeCancelled   = "cancelled"
	CodeBusy        = "busy"
)

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	HTMLURL    string        `json:"html_url"`
	Body       string        `json:"body"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

// Check asks GitHub for the latest stable release and reports it when it is
// newer than current. Pre-releases are never offered.
func Check(ctx context.Context, current, goos string) (Offer, error) {
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if !allowedRedirect(req.URL) {
				return fmt.Errorf("redirect refused")
			}
			return nil
		},
	}
	return check(ctx, current, goos, releaseURL, client)
}

func check(ctx context.Context, current, goos, endpoint string, client *http.Client) (Offer, error) {
	return checkPolicy(ctx, current, goos, endpoint, client, nil)
}

func checkPolicy(ctx context.Context, current, goos, endpoint string, client *http.Client, allow func(string) bool) (Offer, error) {
	if allow == nil {
		allow = acceptedURL
	}
	offer := Offer{Current: strings.TrimSpace(current)}
	if offer.Current == "" {
		return offer, fmt.Errorf("app version is empty")
	}
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return offer, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Norka/"+offer.Current)

	resp, err := client.Do(req)
	if err != nil {
		return offer, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return offer, err
	}
	if resp.StatusCode != http.StatusOK {
		return offer, fmt.Errorf("github releases: %s", resp.Status)
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return offer, err
	}
	offer = offerFromRelease(offer.Current, goos, rel, allow)
	if offer.Available && offer.AssetName != "" && offer.Digest == "" {
		if sumsURL, ok := checksumAssetURL(rel, allow); ok {
			if text, err := fetchText(ctx, client, sumsURL, 256<<10); err == nil {
				offer.Digest = digestForAsset(offer.AssetName, "", text)
				offer.CanApply = canApply(goos, offer.URL, offer.AssetName, offer.Digest)
			}
		}
	}
	if !offer.Available {
		offer.URL = ""
		offer.PageURL = ""
		offer.Notes = ""
		offer.CanApply = false
		offer.Digest = ""
		offer.AssetName = ""
		offer.AssetSize = 0
	}
	return offer, nil
}

func offerFromRelease(current, goos string, rel githubRelease, allow func(string) bool) Offer {
	offer := Offer{Current: strings.TrimSpace(current)}
	offer.Latest = displayVersion(rel.TagName)
	if rel.Draft || rel.Prerelease || isPrereleaseVersion(rel.TagName) {
		return offer
	}
	if !versionNewer(offer.Latest, offer.Current) {
		return offer
	}

	asset, found := selectAsset(goos, rel, allow)
	page := urlIfAllowed(rel.HTMLURL, allow)
	if found {
		offer.URL = asset.URL
		offer.AssetName = asset.Name
		offer.AssetSize = asset.Size
		offer.Digest = asset.Digest
	} else {
		offer.URL = page
	}
	offer.PageURL = page
	offer.Notes = trimNotes(rel.Body)
	offer.Available = offer.URL != ""
	// Linux stays a link: AppImage, then .deb, then the release page. The app is not replaced.
	offer.CanApply = canApply(goos, offer.URL, offer.AssetName, offer.Digest)
	return offer
}

func canApply(goos, downloadURL, assetName, digest string) bool {
	if digest == "" || downloadURL == "" || assetName == "" {
		return false
	}
	switch goos {
	case "windows", "darwin":
		return true
	default:
		return false
	}
}

type selectedAsset struct {
	Name   string
	URL    string
	Size   int64
	Digest string
}

func selectAsset(goos string, rel githubRelease, allow func(string) bool) (selectedAsset, bool) {
	suffixes, exact := assetMatch(goos)
	if len(suffixes) == 0 {
		return selectedAsset{}, false
	}
	// suffixes are in preference order. Linux prefers the AppImage, then a .deb.
	matches := make([]*selectedAsset, len(suffixes))
	for _, asset := range rel.Assets {
		downloadURL := urlIfAllowed(asset.BrowserDownloadURL, allow)
		if downloadURL == "" {
			continue
		}
		name := strings.TrimSpace(asset.Name)
		lower := strings.ToLower(name)
		if lower == "" || isChecksumName(lower) {
			continue
		}
		picked := selectedAsset{
			Name:   name,
			URL:    downloadURL,
			Size:   asset.Size,
			Digest: parseDigest(asset.Digest),
		}
		if exact != "" && lower == exact {
			return picked, true
		}
		for i, suffix := range suffixes {
			if strings.HasSuffix(lower, suffix) && matches[i] == nil {
				copy := picked
				matches[i] = &copy
			}
		}
	}
	for _, match := range matches {
		if match != nil {
			return *match, true
		}
	}
	return selectedAsset{}, false
}

func assetMatch(goos string) (suffixes []string, exact string) {
	switch goos {
	case "windows":
		return []string{".exe"}, "norka.exe"
	case "darwin":
		return []string{".dmg"}, "norka.dmg"
	case "linux":
		return []string{".appimage", ".deb"}, "norka-x86_64.appimage"
	default:
		return nil, ""
	}
}

func isChecksumName(lower string) bool {
	switch lower {
	case "sha256sums", "sha256sums.txt", "checksums.txt":
		return true
	default:
		return false
	}
}

func checksumAssetURL(rel githubRelease, allow func(string) bool) (string, bool) {
	for _, asset := range rel.Assets {
		if !isChecksumName(strings.ToLower(strings.TrimSpace(asset.Name))) {
			continue
		}
		raw := urlIfAllowed(asset.BrowserDownloadURL, allow)
		if raw != "" {
			return raw, true
		}
	}
	return "", false
}

func urlIfAllowed(raw string, allow func(string) bool) string {
	raw = strings.TrimSpace(raw)
	if allow == nil {
		allow = acceptedURL
	}
	if !allow(raw) {
		return ""
	}
	return raw
}

func urlIfAccepted(raw string) string {
	return urlIfAllowed(raw, acceptedURL)
}

func acceptedURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return false
	}
	return strings.HasPrefix(parsed.Path, repoPrefix)
}

func allowedRedirect(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "github.com":
		return strings.HasPrefix(u.Path, repoPrefix)
	case "api.github.com":
		return strings.HasPrefix(u.Path, "/repos/norka-app/Norka/")
	case "release-assets.githubusercontent.com",
		"objects.githubusercontent.com",
		"github-releases.githubusercontent.com":
		return true
	default:
		return false
	}
}

func displayVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	return strings.TrimPrefix(raw, "V")
}

func isPrereleaseVersion(raw string) bool {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	raw = strings.TrimPrefix(raw, "V")
	return strings.Contains(raw, "-")
}

func versionNewer(latest, current string) bool {
	if isPrereleaseVersion(latest) {
		return false
	}
	next, okNext := parseVersion(latest)
	have, okHave := parseVersion(current)
	if !okNext || !okHave {
		return false
	}
	for i := 0; i < len(next); i++ {
		if next[i] > have[i] {
			return true
		}
		if next[i] < have[i] {
			return false
		}
	}
	return false
}

func parseVersion(raw string) ([3]int, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	raw = strings.TrimPrefix(raw, "V")
	if cut := strings.IndexAny(raw, "-+"); cut >= 0 {
		raw = raw[:cut]
	}
	parts := strings.Split(raw, ".")
	if len(parts) == 0 || len(parts) > 3 || raw == "" {
		return [3]int{}, false
	}
	var parsed [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		parsed[i] = n
	}
	return parsed, true
}

func trimNotes(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= notesLimit {
		return raw
	}
	cut := raw[:notesLimit]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func fetchText(ctx context.Context, client *http.Client, rawURL string, limit int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Norka")
	req.Header.Set("Accept", "text/plain")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksums: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SuppressSkipped hides an offer when the user already skipped that version.
// A manual check passes manual=true and is left unchanged.
func SuppressSkipped(offer Offer, skipped string, manual bool) Offer {
	skipped = strings.TrimSpace(skipped)
	if manual || skipped == "" || !offer.Available {
		return offer
	}
	if offer.Latest != skipped {
		return offer
	}
	offer.Available = false
	offer.URL = ""
	offer.PageURL = ""
	offer.Notes = ""
	offer.CanApply = false
	offer.Digest = ""
	offer.AssetName = ""
	offer.AssetSize = 0
	return offer
}
