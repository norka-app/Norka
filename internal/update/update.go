package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	releaseURL = "https://api.github.com/repos/norka-app/Norka/releases/latest"
	repoPrefix = "/norka-app/Norka/"
)

// Offer is the result of comparing the running app with the latest GitHub release.
type Offer struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Check asks GitHub for the latest release and reports it when it is newer
// than current and has a download link for goos.
func Check(ctx context.Context, current, goos string) (Offer, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	return check(ctx, current, goos, releaseURL, client)
}

func check(ctx context.Context, current, goos, endpoint string, client *http.Client) (Offer, error) {
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

	offer.Latest = displayVersion(rel.TagName)
	offer.URL = pickDownloadURL(goos, rel)
	offer.Available = offer.URL != "" && versionNewer(offer.Latest, offer.Current)
	if !offer.Available {
		offer.URL = ""
	}
	return offer, nil
}

func pickDownloadURL(goos string, rel githubRelease) string {
	suffixes, exact := assetMatch(goos)
	if len(suffixes) == 0 {
		return urlIfAccepted(rel.HTMLURL)
	}

	// suffixes are in preference order. Linux prefers the AppImage, then a .deb.
	// The running binary is not replaced: the in-app offer only opens this URL.
	matches := make([]string, len(suffixes))
	for _, asset := range rel.Assets {
		downloadURL := urlIfAccepted(asset.BrowserDownloadURL)
		if downloadURL == "" {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if exact != "" && name == exact {
			return downloadURL
		}
		for i, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) && matches[i] == "" {
				matches[i] = downloadURL
			}
		}
	}
	for _, match := range matches {
		if match != "" {
			return match
		}
	}
	return urlIfAccepted(rel.HTMLURL)
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

func urlIfAccepted(raw string) string {
	raw = strings.TrimSpace(raw)
	if !acceptedURL(raw) {
		return ""
	}
	return raw
}

func acceptedURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") {
		return false
	}
	return strings.HasPrefix(parsed.Path, repoPrefix)
}

func displayVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	return strings.TrimPrefix(raw, "V")
}

func versionNewer(latest, current string) bool {
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
