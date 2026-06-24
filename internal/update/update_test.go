package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithProxy(t *testing.T) {
	got := withProxy("https://api.github.com/x")
	want := "https://ghproxy.homeboyc.cn/https://api.github.com/x"
	if got != want {
		t.Errorf("withProxy = %q; want %q", got, want)
	}
}

func TestMatchAsset(t *testing.T) {
	assets := []Asset{
		{Name: "AndroidFS-macos.dmg", DownloadURL: "https://github.com/dl/mac.dmg"},
		{Name: "AndroidFS-windows-amd64.exe", DownloadURL: "https://github.com/dl/win.exe"},
	}
	cases := []struct {
		goos string
		want string
	}{
		{"darwin", "https://github.com/dl/mac.dmg"},
		{"windows", "https://github.com/dl/win.exe"},
		{"linux", ""}, // no match
	}
	for _, c := range cases {
		got := matchAsset(assets, c.goos)
		if got != c.want {
			t.Errorf("matchAsset(%q) = %q; want %q", c.goos, got, c.want)
		}
	}
}

// releasePayload builds the JSON GitHub's /releases/latest endpoint returns.
func releasePayload(tag, body string, assets []Asset) map[string]any {
	as := make([]map[string]any, len(assets))
	for i, a := range assets {
		as[i] = map[string]any{"name": a.Name, "browser_download_url": a.DownloadURL}
	}
	return map[string]any{"tag_name": tag, "body": body, "assets": as}
}

func TestCheckHasUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releasePayload("v1.2.0", "bug fixes", []Asset{
			{Name: "AndroidFS-macos.dmg", DownloadURL: "https://github.com/dl/mac.dmg"},
		}))
	}))
	defer srv.Close()

	info, err := checkAt(context.Background(), "v1.0.0", "darwin", srv.URL)
	if err != nil {
		t.Fatalf("checkAt error: %v", err)
	}
	if !info.HasUpdate {
		t.Errorf("HasUpdate = false; want true")
	}
	if info.LatestVersion != "v1.2.0" {
		t.Errorf("LatestVersion = %q; want v1.2.0", info.LatestVersion)
	}
	if info.DownloadURL != "https://github.com/dl/mac.dmg" {
		t.Errorf("DownloadURL = %q; want raw github url", info.DownloadURL)
	}
	if info.ReleaseNotes != "bug fixes" {
		t.Errorf("ReleaseNotes = %q; want %q", info.ReleaseNotes, "bug fixes")
	}
}

func TestCheckNoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releasePayload("v1.0.0", "", nil))
	}))
	defer srv.Close()

	info, err := checkAt(context.Background(), "v1.0.0", "darwin", srv.URL)
	if err != nil {
		t.Fatalf("checkAt error: %v", err)
	}
	if info.HasUpdate {
		t.Errorf("HasUpdate = true; want false (same version)")
	}
}

func TestCheckDevSkips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for dev build")
	}))
	defer srv.Close()

	info, err := checkAt(context.Background(), "dev", "darwin", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.HasUpdate {
		t.Errorf("HasUpdate should be false for dev build")
	}
}

func TestCheckAssetMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releasePayload("v1.2.0", "", []Asset{
			{Name: "AndroidFS-windows-amd64.exe", DownloadURL: "https://github.com/dl/win.exe"},
		}))
	}))
	defer srv.Close()

	info, err := checkAt(context.Background(), "v1.0.0", "darwin", srv.URL)
	if err != nil {
		t.Fatalf("checkAt error: %v", err)
	}
	if !info.HasUpdate {
		t.Fatalf("HasUpdate should be true")
	}
	if info.DownloadURL != "" {
		t.Errorf("DownloadURL = %q; want empty (no mac asset)", info.DownloadURL)
	}
}
