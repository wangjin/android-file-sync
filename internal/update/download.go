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
