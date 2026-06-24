package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ProxyPrefix is prepended to GitHub URLs so mainland-China users can reach
// them; requests fall back to a direct connection if the proxy fails.
const ProxyPrefix = "https://ghproxy.homeboyc.cn/"

// GitHubReleaseAPI is the endpoint that returns the latest release JSON.
const GitHubReleaseAPI = "https://api.github.com/repos/wangjin/android-file-sync/releases/latest"

// Info describes the result of an update check.
type Info struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	DownloadURL    string `json:"download_url"` // raw GitHub URL; proxy applied at download time
	ReleaseNotes   string `json:"release_notes"`
}

// Asset is one downloadable file attached to a GitHub release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

// withProxy prepends the ghproxy mirror prefix to a raw GitHub URL.
func withProxy(rawURL string) string {
	return ProxyPrefix + rawURL
}

// matchAsset returns the browser_download_url for the asset matching goos, or
// "" if none matches. macOS -> *.dmg, Windows -> *.exe, Linux -> none.
func matchAsset(assets []Asset, goos string) string {
	var suffix string
	switch goos {
	case "darwin":
		suffix = ".dmg"
	case "windows":
		suffix = ".exe"
	default:
		return ""
	}
	for _, a := range assets {
		if len(a.Name) >= len(suffix) && a.Name[len(a.Name)-len(suffix):] == suffix {
			return a.DownloadURL
		}
	}
	return ""
}

// checkAt queries a single release endpoint (proxy or direct) and returns Info.
// A "dev" current version short-circuits to no-update without a request.
func checkAt(ctx context.Context, current, goos, endpoint string) (*Info, error) {
	info := &Info{CurrentVersion: current}
	if current == "" || current == "dev" {
		return info, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("release check: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return info, fmt.Errorf("release parse: %w", err)
	}

	info.LatestVersion = rel.TagName
	info.ReleaseNotes = rel.Body
	if Compare(current, rel.TagName) < 0 {
		info.HasUpdate = true
		info.DownloadURL = matchAsset(rel.Assets, goos)
	}
	return info, nil
}

// Check queries the latest GitHub release via the proxy first, falling back to a
// direct connection. Returns Info (possibly with HasUpdate=false) and an error
// only when both endpoints fail.
func Check(ctx context.Context, current, goos string) (*Info, error) {
	if info, err := checkAt(ctx, current, goos, withProxy(GitHubReleaseAPI)); err == nil {
		return info, nil
	}
	// proxy failed: try direct
	return checkAt(ctx, current, goos, GitHubReleaseAPI)
}
