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
