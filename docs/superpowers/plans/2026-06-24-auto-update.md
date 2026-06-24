# 版本号显示与自动更新功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 AndroidFS 中显示当前版本号，启动时自动检查 GitHub Release 更新，发现新版本时弹提示，用户点击后应用内下载（带进度条）并自动打开对应平台安装包。

**Architecture:** Go 后端驱动。所有更新逻辑（版本比较、GitHub API 检查、下载、打开安装包）集中在新增的 `internal/update` 包，通过 `app_update.go` 暴露为 `App` 方法，前端为纯 UI 层（调用方法 + 监听 Wails 事件）。请求通过 ghproxy 加速，代理失败回退直连。

**Tech Stack:** Go 1.25（标准库 `net/http`、`os/exec`）、Wails v3（`application.Event`）、React + TypeScript 前端、`wails3 generate bindings -ts` 自动生成前端绑定。

## Global Constraints

- **版本变量已存在**：`main.version`（`main.go:10`）构建时由 `git describe --tags --always` 注入，值形如 `v1.0.0`、`v1.0.0-3-gabc123` 或 `dev`。
- **平台检测已有**：`runtimeGOOS()`（`app_runtime.go:8`）返回 `darwin`/`windows`/`linux`。
- **GitHub 仓库**：`wangjin/android-file-sync`。
- **加速代理前缀（硬编码常量）**：`https://ghproxy.homeboyc.cn/`。请求时拼在目标 URL 前：`proxy + targetURL`（targetURL 保留 `https://`）。
- **Release 资产命名**：macOS = `AndroidFS-macos.dmg`，Windows = `AndroidFS-windows-amd64.exe`。
- **GitHub API 端点**：`https://api.github.com/repos/wangjin/android-file-sync/releases/latest`。
- **测试命令**：`go test ./internal/update/... -v -race`（沿用 `Taskfile.yml` 的 `test` 任务模式）。
- **前端绑定生成**：`wails3 generate bindings -ts -d frontend/bindings .`（见 `Taskfile.yml` generate 任务）。每个新增/改动的 `App` 方法任务后必须重新生成绑定。
- **设计系统**：暗色主题。accent = `--signal`（mint `#4ADE9E`），danger = `--vermilion`，mist = `--mist`（次要文字），frost = `--frost`（主文字）。`button.primary` 用 signal 色，`.mono` 用于等宽数值。CSS 在 `frontend/src/style.css`，tokens 在 `frontend/src/styles/tokens.css`。
- **commit 消息**：遵循现有 conventional 风格（`feat:`、`fix:`、`docs:`、`test:`）。

---

## File Structure

| 文件 | 责任 | 动作 |
|------|------|------|
| `internal/update/version.go` | 版本解析与比较（纯函数） | 创建 |
| `internal/update/version_test.go` | 版本比较表驱动测试 | 创建 |
| `internal/update/update.go` | GitHub API 检查、资产匹配、代理 URL 构造、`Info` 结构 | 创建 |
| `internal/update/update_test.go` | 资产匹配、代理 URL、Check 的 httptest mock 测试 | 创建 |
| `internal/update/download.go` | 带进度的下载，写到临时文件，带重试 | 创建 |
| `internal/update/download_test.go` | Download 的 httptest mock 测试 | 创建 |
| `internal/update/open.go` | 平台分发打开安装包 | 创建 |
| `app_update.go` | `App` 层方法：`Version`/`CheckUpdate`/`DownloadUpdate`，事件发射 | 创建 |
| `app.go` | `ServiceStartup` 启动延迟自动检查 | 修改 |
| `frontend/src/components/UpdateDialog.tsx` | 更新提示 + 下载进度弹窗 | 创建 |
| `frontend/src/components/Toast.tsx` | 轻量提示（手动检查反馈） | 创建 |
| `frontend/src/hooks/useUpdate.ts` | 自动检查订阅 + 事件监听 hook | 创建 |
| `frontend/src/App.tsx` | 挂载 hook、渲染弹窗/toast | 修改 |
| `frontend/src/components/Toolbar.tsx` | 显示版本号、点击手动检查 | 修改 |
| `frontend/src/style.css` | 版本号、进度条、toast 样式 | 修改 |

**依赖顺序**：version → update(Check) → download → open → app_update → backend wiring(app.go) → frontend。

---

## Task 1: 版本解析与比较（version.go）

**Files:**
- Create: `internal/update/version.go`
- Create: `internal/update/version_test.go`

**Interfaces:**
- Consumes: 无（纯函数）
- Produces:
  - `Parse(v string) (Version, bool)` — `Version` 是 `[3]int{major,minor,patch}`
  - `Compare(a, b string) int` — -1 表示 a<b（a 旧），0 相等，1 表示 a>b（a 新）

- [ ] **Step 1: Write the failing test**

Create `internal/update/version_test.go`:

```go
package update

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"v1.0.0", [3]int{1, 0, 0}, true},
		{"1.0.0", [3]int{1, 0, 0}, true},        // no v prefix
		{"1.0", [3]int{1, 0, 0}, true},          // missing patch -> 0
		{"2", [3]int{2, 0, 0}, true},            // only major
		{"v1.0.0-3-gabc123", [3]int{1, 0, 0}, true}, // git describe format
		{"dev", [3]int{}, false},                // dev build
		{"garbage", [3]int{}, false},            // non-numeric
		{"", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("Parse(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.9", 1},
		{"1.0.0", "v1.0.0", 0},     // equal ignoring prefix
		{"v2.0.0", "v1.9.9", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v1.0.0", "v1.0.0", 0},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		// normalize: only sign matters
		sign := 0
		if got < 0 {
			sign = -1
		} else if got > 0 {
			sign = 1
		}
		if sign != c.want {
			t.Errorf("Compare(%q,%q) sign = %d; want %d", c.a, c.b, sign, c.want)
		}
	}
}

// Compare must not panic on non-semver input — it falls back to string compare.
func TestCompareNonSemver(t *testing.T) {
	// "dev" vs "v1.0.0": dev not parseable, falls back to lexical.
	// Must not panic; returns some int.
	got := Compare("dev", "v1.0.0")
	if got != -1 && got != 0 && got != 1 {
		t.Errorf("Compare(dev, v1.0.0) = %d, want one of -1/0/1", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/update/... -run 'TestParse|TestCompare' -v`
Expected: FAIL — package not found / functions undefined (build error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/update/version.go`:

```go
// Package update handles checking for, downloading, and opening app updates
// from GitHub Releases. Network access goes through the ghproxy mirror first,
// falling back to a direct connection.
package update

import (
	"log"
	"strconv"
	"strings"
)

// Version is a parsed semantic version: [major, minor, patch].
type Version [3]int

// Parse turns a version string into a Version. Accepts an optional leading "v",
// missing segments (treated as 0), and a trailing "-N-gHASH" suffix produced by
// `git describe`. Returns ok=false for anything it cannot parse (e.g. "dev",
// non-numeric segments), without panicking.
func Parse(v string) (Version, bool) {
	v = strings.TrimPrefix(v, "v")
	// strip git-describe suffix: "1.0.0-3-gabc123" -> "1.0.0"
	if i := strings.Index(v, "-"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, false
	}
	var out Version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, false
		}
		if n < 0 {
			return Version{}, false
		}
		out[i] = n
	}
	return out, true
}

// Compare returns -1 if a is older than b, 0 if equal, 1 if a is newer.
// If either string cannot be parsed, it logs the fallback and compares the raw
// strings lexically (never panicking).
func Compare(a, b string) int {
	va, oka := Parse(a)
	vb, okb := Parse(b)
	if !oka || !okb {
		log.Printf("update: version parse fallback (a=%q ok=%v, b=%q ok=%v), using lexical compare", a, oka, b, okb)
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}
	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1
		}
		if va[i] > vb[i] {
			return 1
		}
	}
	return 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/update/... -run 'TestParse|TestCompare' -v`
Expected: PASS — all subtests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/update/version.go internal/update/version_test.go
git commit -m "feat(update): version parse and semver compare with test"
```

---

## Task 2: GitHub API 检查与资产匹配（update.go）

**Files:**
- Create: `internal/update/update.go`
- Create: `internal/update/update_test.go`

**Interfaces:**
- Consumes: `Compare(a, b string) int` from Task 1
- Produces:
  - `type Info struct` — exported fields `CurrentVersion, LatestVersion, HasUpdate, DownloadURL, ReleaseNotes string`
  - `Check(ctx context.Context, current, goos string) (*Info, error)` — `current` is the running version; `goos` selects the asset. Returns `Info` always (on no-update, `HasUpdate=false`); returns error only on total network failure.
  - `ProxyPrefix` (constant) and `withProxy(rawURL string) string` — helper (tested directly).

- [ ] **Step 1: Write the failing test**

Create `internal/update/update_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/update/... -run 'TestWithProxy|TestMatchAsset|TestCheck' -v`
Expected: FAIL — build errors (`withProxy`, `matchAsset`, `Info`, `Asset`, `checkAt` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/update/update.go`:

```go
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
	Name         string `json:"name"`
	DownloadURL  string `json:"browser_download_url"`
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/update/... -run 'TestWithProxy|TestMatchAsset|TestCheck' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): GitHub release check with asset matching and proxy"
```

---

## Task 3: 带进度的下载（download.go）

**Files:**
- Create: `internal/update/download.go`
- Create: `internal/update/download_test.go`

**Interfaces:**
- Consumes: `withProxy(rawURL string) string` from Task 2
- Produces:
  - `type Progress struct{ Percent, Downloaded, Total int64 }`
  - `type ProgressFn func(Progress)` — passed by caller, throttled to 500ms
  - `Download(ctx context.Context, rawURL string, onProgress ProgressFn) (string, error)` — downloads via proxy-then-direct, to a temp file under the OS cache dir, returns the local path. Retries once on failure.

- [ ] **Step 1: Write the failing test**

Create `internal/update/download_test.go`:

```go
package update

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDownloadWritesFileAndReportsProgress(t *testing.T) {
	payload := strings.Repeat("x", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = io.WriteString(w, payload)
	}))
	defer srv.Close()

	var last Progress
	path, err := Download(context.Background(), srv.URL, func(p Progress) {
		last = p
	})
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if path == "" {
		t.Fatal("empty path returned")
	}
	// last progress should reflect full download
	if last.Total != 1000 {
		t.Errorf("last.Total = %d; want 1000", last.Total)
	}
	if last.Percent != 100 {
		t.Errorf("last.Percent = %d; want 100", last.Percent)
	}
	// file contents correct
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(data) != 1000 {
		t.Errorf("downloaded %d bytes; want 1000", len(data))
	}
}

func TestDownloadRetriesOnFailure(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", "5")
		_, _ = io.WriteString(w, "hello")
	}))
	defer srv.Close()

	path, err := Download(context.Background(), srv.URL, func(Progress) {})
	if err != nil {
		t.Fatalf("Download after retry error: %v", err)
	}
	if calls != 2 {
		t.Errorf("server called %d times; want 2 (retry)", calls)
	}
	if path == "" {
		t.Fatal("empty path after successful retry")
	}
}

func TestDownloadAllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Download(context.Background(), srv.URL, func(Progress) {})
	if err == nil {
		t.Fatal("expected error when all attempts fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/update/... -run 'TestDownload' -v`
Expected: FAIL — `Download`, `Progress`, `ProgressFn` undefined (build error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/update/download.go`:

```go
package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Progress is reported to the caller during a download. Percent is 0..100.
type Progress struct {
	Percent    int64
	Downloaded int64
	Total      int64
}

// ProgressFn is called (throttled to ~500ms) as bytes arrive.
type ProgressFn func(Progress)

// downloadOnce performs a single HTTP GET to endpoint, streaming the body to a
// temp file and reporting progress via onProgress.
func downloadOnce(ctx context.Context, endpoint string, onProgress ProgressFn) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	path, err := tempPath(endpoint)
	if err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	total := resp.ContentLength
	pr := &progressReader{reader: resp.Body, total: total, onProgress: onProgress, last: time.Now()}
	if _, err := io.Copy(f, pr); err != nil {
		os.Remove(path)
		return "", err
	}
	// final 100% report
	if onProgress != nil {
		onProgress(Progress{Percent: 100, Downloaded: pr.read, Total: total})
	}
	return path, nil
}

// Download fetches rawURL (a GitHub asset URL) to a temp file, proxy first then
// direct, retrying once per endpoint on failure. onProgress is throttled to
// 500ms. Returns the local file path.
func Download(ctx context.Context, rawURL string, onProgress ProgressFn) (string, error) {
	endpoints := []string{withProxy(rawURL), rawURL}
	var lastErr error
	for _, ep := range endpoints {
		// one retry per endpoint
		for attempt := 0; attempt < 2; attempt++ {
			path, err := downloadOnce(ctx, ep, onProgress)
			if err == nil {
				return path, nil
			}
			lastErr = err
		}
	}
	return "", lastErr
}

// progressReader wraps a reader, reporting download progress at most every
// 500ms to avoid flooding the frontend with events.
type progressReader struct {
	reader     io.Reader
	total      int64
	read       int64
	onProgress ProgressFn
	last       time.Time
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	p.read += int64(n)
	if p.onProgress != nil && time.Since(p.last) >= 500*time.Millisecond {
		pct := int64(0)
		if p.total > 0 {
			pct = p.read * 100 / p.total
		}
		p.onProgress(Progress{Percent: pct, Downloaded: p.read, Total: p.total})
		p.last = time.Now()
	}
	return n, err
}

// tempPath returns a path under the OS cache dir for the downloaded asset,
// preserving the file extension from the URL.
func tempPath(rawURL string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "AndroidFS")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(rawURL)
	if ext == "" {
		ext = ".bin"
	}
	f, err := os.CreateTemp(dir, "update-*"+ext)
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	return name, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/update/... -run 'TestDownload' -v`
Expected: PASS — file written, progress reported, retry works, all-fail errors.

- [ ] **Step 5: Commit**

```bash
git add internal/update/download.go internal/update/download_test.go
git commit -m "feat(update): throttled download with retry to temp file"
```

---

## Task 4: 打开安装包（open.go）

**Files:**
- Create: `internal/update/open.go`

**Interfaces:**
- Consumes: `runtimeGOOS()` (existing, `app_runtime.go:8`)
- Produces: `Open(path string) error` — opens an installer with the OS default handler (`open` on macOS, `cmd /c start` on Windows).

> Note: `Open` involves a system call; it is exercised by manual verification, not a unit test. Keep it a thin, obviously-correct wrapper.

- [ ] **Step 1: Write the implementation**

Create `internal/update/open.go`:

```go
package update

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open opens the downloaded installer with the OS default application: Finder
// mounts a .dmg on macOS, Windows runs / opens a .exe. Returns an error if the
// platform is unsupported or the launch command fails.
func Open(path string) error {
	switch runtime.GOOS {
	case "darwin":
		// `open` reveals/launches the file via Finder/LaunchServices.
		if err := exec.Command("open", path).Start(); err != nil {
			return fmt.Errorf("open installer: %w", err)
		}
		return nil
	case "windows":
		// `start "" <path>` opens with the default handler; the empty title
		// arg prevents the path being consumed as the console-window title.
		if err := exec.Command("cmd", "/c", "start", "", path).Start(); err != nil {
			return fmt.Errorf("open installer: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("open installer: unsupported platform %s", runtime.GOOS)
	}
}
```

- [ ] **Step 2: Verify it compiles and the package lints**

Run: `go build ./internal/update/... && go vet ./internal/update/...`
Expected: no output (success).

- [ ] **Step 3: Run the full update package test suite to confirm nothing broke**

Run: `go test ./internal/update/... -v -race`
Expected: PASS — all prior tests still green.

- [ ] **Step 4: Commit**

```bash
git add internal/update/open.go
git commit -m "feat(update): open installer via os default handler (macos/windows)"
```

---

## Task 5: App 层方法与事件（app_update.go）

**Files:**
- Create: `app_update.go`
- Modify: `app.go` (启动自动检查)

**Interfaces:**
- Consumes:
  - `main.version` (package-level, `main.go:10`)
  - `runtimeGOOS()` (`app_runtime.go:8`)
  - `update.Check`, `update.Download`, `update.Open`, `update.Info`, `update.ProgressFn` (Tasks 2-4)
- Produces (exposed to frontend via bindings):
  - `App.Version() string`
  - `App.CheckUpdate() (*update.Info, error)`
  - `App.DownloadUpdate(url string) error` — async; emits `update:progress`/`update:done`/`update:error`

- [ ] **Step 1: Write app_update.go**

Create `app_update.go`:

```go
package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"androidfs/internal/update"
)

// Version returns the running application version (injected at build time, or
// "dev" in development).
func (a *App) Version() string { return version }

// CheckUpdate queries GitHub for the latest release and returns whether a newer
// version exists. The frontend uses this for manual checks; the startup auto-
// check calls it via autoCheck (below) and emits update:available.
func (a *App) CheckUpdate() (*update.Info, error) {
	return update.Check(a.ctx, version, runtimeGOOS())
}

// DownloadUpdate downloads the given GitHub asset URL to a temp file, emitting
// update:progress events as it goes. On completion it opens the installer and
// emits update:done; on failure it emits update:error. Runs in a goroutine so
// the frontend call returns immediately.
func (a *App) DownloadUpdate(url string) error {
	go func() {
		path, err := update.Download(a.ctx, url, func(p update.Progress) {
			application.Get().Event.Emit("update:progress", p)
		})
		if err != nil {
			log.Printf("update download failed: %v", err)
			application.Get().Event.Emit("update:error", map[string]any{"message": err.Error()})
			return
		}
		if err := update.Open(path); err != nil {
			log.Printf("update open failed: %v", err)
			application.Get().Event.Emit("update:error", map[string]any{"message": err.Error()})
			return
		}
		application.Get().Event.Emit("update:done", map[string]any{"path": path})
	}()
	return nil
}

// autoCheck runs once shortly after startup; on a positive result it emits
// update:available so the frontend can show the prompt. Failures are silent.
func (a *App) autoCheck(ctx context.Context) {
	info, err := update.Check(ctx, version, runtimeGOOS())
	if err != nil {
		log.Printf("startup update check failed (silent): %v", err)
		return
	}
	if info.HasUpdate {
		application.Get().Event.Emit("update:available", info)
	}
}
```

- [ ] **Step 2: Wire the startup auto-check into ServiceStartup**

Modify `app.go`. In `ServiceStartup`, after `go a.pollDevices(devCtx)` (currently line 50), add the delayed auto-check. The existing import block already imports `time`; add `time.AfterFunc`.

Replace this block in `app.go` (the tail of `ServiceStartup`):

```go
	devCtx, cancel := context.WithCancel(ctx)
	a.cancelDev = cancel
	go a.pollDevices(devCtx)
	return nil
}
```

with:

```go
	devCtx, cancel := context.WithCancel(ctx)
	a.cancelDev = cancel
	go a.pollDevices(devCtx)

	// Auto-check for updates 3s after startup, off the critical launch path.
	// adb setup and device polling have already begun by then.
	time.AfterFunc(3*time.Second, func() { a.autoCheck(devCtx) })
	return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output (success).

- [ ] **Step 4: Regenerate frontend bindings**

Run: `wails3 generate bindings -ts -d frontend/bindings .`
Expected: `frontend/bindings/androidfs/app.ts` now contains `Version`, `CheckUpdate`, `DownloadUpdate` exports. A new `update` model file appears under `frontend/bindings/androidfs/internal/update/`.

Verify (read `frontend/bindings/androidfs/app.ts`): confirm `export function Version()`, `export function CheckUpdate()`, `export function DownloadUpdate(...)` are present.

- [ ] **Step 5: Commit**

```bash
git add app_update.go app.go frontend/bindings/
git commit -m "feat(update): app-layer methods, startup auto-check, bindings"
```

---

## Task 6: 前端 useUpdate hook + Toast 组件

**Files:**
- Create: `frontend/src/hooks/useUpdate.ts`
- Create: `frontend/src/components/Toast.tsx`
- Modify: `frontend/src/style.css` (toast + version styles)

**Interfaces:**
- Consumes: `Events` from `@wailsio/runtime`; generated bindings `CheckUpdate` from `../bindings/androidfs/app.js`
- Produces:
  - `useUpdate()` returning `{ info, toast, dismissToast, checkNow }` — `info` is `UpdateInfo|null` (when an update is available), `toast` is a transient message string, `checkNow()` triggers manual check.

- [ ] **Step 1: Write the Toast component**

Create `frontend/src/components/Toast.tsx`:

```tsx
// A lightweight, auto-dismissing notification used for non-blocking feedback
// (e.g. manual update check: "已是最新版本"). It floats at the bottom-center.
export function Toast({ message }: { message: string | null }) {
  if (!message) return null
  return (
    <div className="toast" role="status">{message}</div>
  )
}
```

- [ ] **Step 2: Add toast + version styles**

Append to `frontend/src/style.css` (after the context-menu section, before the "Disable native text selection" comment block):

```css
/* ===== Toast ===== */
.toast {
  position: fixed;
  bottom: var(--space-6);
  left: 50%;
  transform: translateX(-50%);
  background: var(--ink-700);
  border: 1px solid var(--ink-600);
  color: var(--frost);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  font-size: var(--fs-sm);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  z-index: 1100;
}

/* ===== Version label (toolbar) ===== */
.version {
  font-size: var(--fs-xs);
  color: var(--mist);
  cursor: pointer;
  padding: 2px var(--space-1);
  border-radius: var(--radius-sm);
  transition: color 120ms ease, background 120ms ease;
}
.version:hover { color: var(--frost); background: var(--ink-700); }
```

- [ ] **Step 3: Write the useUpdate hook**

Create `frontend/src/hooks/useUpdate.ts`:

```ts
import { useState, useEffect, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import { CheckUpdate } from '../../bindings/androidfs/app.js'
import type { Info as UpdateInfo } from '../../bindings/androidfs/internal/update/models.js'

// useUpdate subscribes to the backend's startup auto-check (update:available)
// and exposes a manual checkNow(). Toast messages are surfaced for manual
// feedback ("已是最新版本" / error); the auto-check is silent on failure.
export function useUpdate() {
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const timer = useRef<number | null>(null)

  const flash = (msg: string) => {
    setToast(msg)
    if (timer.current) window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setToast(null), 3000)
  }

  // auto-check result from backend
  useEffect(() => {
    const off = Events.On('update:available', (ev: any) => {
      if (ev.data) setInfo(ev.data as UpdateInfo)
    })
    return () => { off() }
  }, [])

  // manual check: always gives feedback
  const checkNow = async () => {
    try {
      const res = await CheckUpdate()
      if (res && res.has_update) {
        setInfo(res)
      } else {
        flash('已是最新版本')
      }
    } catch (e: any) {
      flash('检查更新失败，请检查网络')
    }
  }

  const dismissToast = () => setToast(null)
  const dismissInfo = () => setInfo(null)

  return { info, toast, dismissToast, dismissInfo, checkNow }
}
```

> **Note on the Info import path:** the exact generated path for the `Info` type may be `frontend/bindings/androidfs/internal/update/models.js` (Task 5 generates it). If the build fails to resolve it, run `wails3 generate bindings -ts -d frontend/bindings .` again and adjust the import to match the actual emitted path. The runtime binding `CheckUpdate` resolves from `app.js` regardless.

- [ ] **Step 4: Verify the frontend type-checks**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors. (If the `Info` import path is wrong, fix it per the note above.)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/hooks/useUpdate.ts frontend/src/components/Toast.tsx frontend/src/style.css
git commit -m "feat(ui): useUpdate hook and Toast component for update feedback"
```

---

## Task 7: UpdateDialog 组件 + Toolbar 版本号

**Files:**
- Create: `frontend/src/components/UpdateDialog.tsx`
- Modify: `frontend/src/components/Toolbar.tsx`
- Modify: `frontend/src/style.css` (progress bar + dialog styles)

**Interfaces:**
- Consumes: `useUpdate()` `info` (UpdateInfo), generated bindings `DownloadUpdate` from `../bindings/androidfs/app.js`, `Events.On('update:progress'|'update:done'|'update:error')`
- Produces: `UpdateDialog` (renders prompt + in-place progress), and a clickable version label in the Toolbar.

- [ ] **Step 1: Add progress + update-dialog styles**

Append to `frontend/src/style.css` (after the version styles added in Task 6):

```css
/* ===== Update dialog ===== */
.update-notes {
  color: var(--mist);
  font-size: var(--fs-sm);
  line-height: 1.5;
  max-height: 140px;
  overflow: auto;
  margin: 0 0 var(--space-2);
  white-space: pre-wrap;
}
.update-version { color: var(--signal); font-weight: var(--fw-semibold); }
.update-progress-track {
  height: 6px;
  border-radius: 3px;
  background: var(--ink-900);
  overflow: hidden;
  margin-top: var(--space-2);
}
.update-progress-fill {
  height: 100%;
  background: var(--signal);
  transition: width 300ms ease;
}
.update-pct { font-size: var(--fs-xs); color: var(--mist); margin-top: var(--space-1); }
.update-error { color: var(--vermilion); font-size: var(--fs-sm); margin-top: var(--space-2); }
```

- [ ] **Step 2: Write UpdateDialog**

Create `frontend/src/components/UpdateDialog.tsx`:

```tsx
import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { DownloadUpdate } from '../../bindings/androidfs/app.js'
import type { Info as UpdateInfo } from '../../bindings/androidfs/internal/update/models.js'

// UpdateDialog shows when an update is available. Clicking 立即下载 starts an
// in-app download with a live progress bar; on completion the backend opens the
// installer and emits update:done. Errors surface inline with a retry.
export function UpdateDialog({ info, onClose }: {
  info: UpdateInfo
  onClose: () => void
}) {
  const [phase, setPhase] = useState<'prompt' | 'downloading' | 'error'>('prompt')
  const [percent, setPercent] = useState(0)
  const [error, setError] = useState('')

  useEffect(() => {
    const offP = Events.On('update:progress', (ev: any) => {
      setPercent(ev.data?.percent ?? 0)
    })
    const offD = Events.On('update:done', () => {
      onClose()   // installer opened; close the dialog
    })
    const offE = Events.On('update:error', (ev: any) => {
      setError(ev.data?.message ?? '下载失败')
      setPhase('error')
    })
    return () => { offP(); offD(); offE() }
  }, [onClose])

  const start = async () => {
    setPhase('downloading')
    setPercent(0)
    await DownloadUpdate(info.download_url)
  }

  return (
    <div className="overlay" role="dialog" aria-modal="true">
      <div className="dialog">
        <h3 className="dialog-title">发现新版本</h3>
        {phase === 'prompt' && (
          <>
            <div className="confirm-message">
              当前 <span className="mono">{info.current_version}</span>，最新
              <span className="update-version mono"> {info.latest_version}</span>。
            </div>
            {info.release_notes && <div className="update-notes">{info.release_notes}</div>}
            <div className="dialog-actions">
              <button onClick={onClose}>稍后</button>
              <button
                className="primary"
                disabled={!info.download_url}
                onClick={start}
              >
                {info.download_url ? '立即下载' : '当前平台暂无安装包'}
              </button>
            </div>
          </>
        )}
        {phase === 'downloading' && (
          <>
            <div className="confirm-message">正在下载…</div>
            <div className="update-progress-track">
              <div className="update-progress-fill" style={{ width: `${percent}%` }} />
            </div>
            <div className="update-pct mono">{percent}%</div>
          </>
        )}
        {phase === 'error' && (
          <>
            <div className="confirm-message">下载失败</div>
            <div className="update-error">{error}</div>
            <div className="dialog-actions">
              <button onClick={onClose}>关闭</button>
              <button className="primary" disabled={!info.download_url} onClick={start}>重试</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Wire version label into Toolbar**

Modify `frontend/src/components/Toolbar.tsx`. Add a `version` and `onCheckUpdate` prop and render a clickable label after the brand.

Replace the full file content:

```tsx
import { Device } from '../../bindings/androidfs/internal/model/models.js'

export function Toolbar({ devices, selected, onSelect, onRefresh, onConnect, version, onCheckUpdate }: {
  devices: Device[]
  selected: string | null
  onSelect: (s: string) => void
  onRefresh: () => void
  onConnect: () => void
  version: string
  onCheckUpdate: () => void
}) {
  const warn = selected && devices.find(d => d.serial === selected)?.state === 'unauthorized'
  return (
    <header className="toolbar">
      <span className="brand mono">AndroidFS</span>
      <span
        className="version mono"
        title="点击检查更新"
        onClick={onCheckUpdate}
      >
        {version === 'dev' ? 'dev' : version}
      </span>
      <select className="device-select" value={selected ?? ''} onChange={e => onSelect(e.target.value)}>
        <option value="" disabled>选择设备…</option>
        {devices.map(d => (
          <option key={d.serial} value={d.serial}>
            {d.model || d.serial} · {d.transport} · {d.state}
          </option>
        ))}
      </select>
      {warn && <span className="warn mono">在设备上允许 USB 调试授权</span>}
      <span className="toolbar-spacer" />
      <button onClick={onRefresh}>刷新</button>
      <button onClick={onConnect}>无线连接</button>
    </header>
  )
}
```

- [ ] **Step 4: Verify the frontend type-checks**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/UpdateDialog.tsx frontend/src/components/Toolbar.tsx frontend/src/style.css
git commit -m "feat(ui): UpdateDialog with progress, clickable version in toolbar"
```

---

## Task 8: App.tsx 集成

**Files:**
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `useUpdate()` (Task 6), `UpdateDialog` (Task 7), `Toast` (Task 6), `Version` binding, updated `Toolbar`
- Produces: fully wired update UI in the running app.

- [ ] **Step 1: Wire useUpdate into App.tsx**

Modify `frontend/src/App.tsx`. Add imports and hook usage, pass props to Toolbar, render the dialog + toast.

Add these imports near the existing component imports (after the `EmptyState` import, line 15):

```tsx
import { Toast } from './components/Toast'
import { UpdateDialog } from './components/UpdateDialog'
import { useUpdate } from './hooks/useUpdate'
import { Version } from '../bindings/androidfs/app.js'
```

Inside the `App` function, after the `const device = useDeviceBrowser(serial)` line, add:

```tsx
  const update = useUpdate()
  const [appVersion, setAppVersion] = useState('dev')
  useEffect(() => { Version().then(setAppVersion) }, [])
```

Update the `<Toolbar ... />` JSX to pass the new props (add after `onConnect={...}`):

```tsx
        version={appVersion}
        onCheckUpdate={update.checkNow}
```

Finally, add the dialog and toast before the closing `</div>` of `app-root` (after the `ConfirmDialog`):

```tsx
      {update.info && (
        <UpdateDialog info={update.info} onClose={update.dismissInfo} />
      )}
      <Toast message={update.toast} />
```

- [ ] **Step 2: Verify the frontend type-checks and builds**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: build succeeds, `frontend/dist/index.html` regenerated.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(ui): wire update check, prompt dialog, and version display"
```

---

## Task 9: 端到端构建验证与手动验证清单

**Files:** 无新文件 — 验证任务。

- [ ] **Step 1: Full backend test suite**

Run: `go test ./internal/update/... -v -race`
Expected: PASS — all version/update/download tests green.

- [ ] **Step 2: Full Go build (current platform)**

Run: `go build ./...`
Expected: no output (success).

- [ ] **Step 3: go vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 4: Build the app via Taskfile**

Run: `task build`
Expected: `build/bin/AndroidFS` produced with version injected (verify with the running app showing the version in the toolbar).

- [ ] **Step 5: Manual verification (run the built app)**

Run: `./build/bin/AndroidFS` (or `task dev`)
Verify against the spec's manual checklist:
1. Toolbar shows `AndroidFS` + version label (e.g. `v…` or `dev` in dev mode).
2. Click the version label → manual check runs → if no release, toast "已是最新版本"; if network down, toast "检查更新失败，请检查网络".
3. With a GitHub release whose tag > current version: the prompt dialog appears (auto at startup, or on manual check) showing new version + notes.
4. Click 立即下载 → progress bar advances → on completion the installer opens (.dmg on macOS).
5. No-platform-asset case (if testable): button shows "当前平台暂无安装包" and is disabled.

- [ ] **Step 6: Commit any final binding/build artifacts if changed**

```bash
git add -A
git commit -m "chore: regenerated bindings/build artifacts" || echo "nothing to commit"
```

---

## Notes for the Implementer

- **Binding regeneration is mandatory** after adding/changing any `App` method (Tasks 5). The frontend imports are auto-generated; if an import path differs from what a task assumes, regenerate and match the actual emitted path.
- **`main.version` is a package-level var** in `main.go`; `app_update.go` (also `package main`) references it directly — no import needed.
- **`time` is already imported** in `app.go`; the `time.AfterFunc` addition needs no new import.
- **ghproxy prefix is a constant** in `update.go` (`ProxyPrefix`). Never accept it from external input.
- **The auto-check must stay silent on failure** — only `update:available` is emitted on a positive result. Manual checks (`checkNow`) always give feedback.
