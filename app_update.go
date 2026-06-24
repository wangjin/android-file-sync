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
//
// Starting a new download cancels any in-flight one (tracked on the App) so the
// user can't launch two concurrent downloads by retrying mid-stream. Each
// download gets a generation id; only the latest generation reports errors/
// completion, so a superseded download stays silent.
func (a *App) DownloadUpdate(url string) error {
	a.mu.Lock()
	if a.cancelDownload != nil {
		a.cancelDownload() // tear down any prior download
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.downloadGen++
	gen := a.downloadGen
	a.cancelDownload = cancel
	a.mu.Unlock()

	go func() {
		defer cancel()

		path, err := update.Download(ctx, url, func(p update.Progress) {
			application.Get().Event.Emit("update:progress", p)
		})

		// Determine whether we are still the active download. If a newer one
		// started, stay silent — its goroutine owns the UI now. Only the latest
		// generation (by counter, not by func identity) clears the cancel func.
		a.mu.Lock()
		stillActive := a.downloadGen == gen
		if stillActive {
			a.cancelDownload = nil
		}
		a.mu.Unlock()

		if err != nil {
			if ctx.Err() != nil || !stillActive {
				return // cancelled/superseded — not worth surfacing
			}
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
