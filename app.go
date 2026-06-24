package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"androidfs/internal/adb"
	"androidfs/internal/model"
	"androidfs/internal/queue"
	"androidfs/internal/transfer"
)

type App struct {
	ctx       context.Context
	client    *adb.AdbClient
	queue     *queue.Manager
	engine    *transfer.Engine
	cancelDev context.CancelFunc
	mu        sync.Mutex
}

func NewApp() *App { return &App{} }

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	bin, err := ensureAdbBinary()
	if err != nil {
		return fmt.Errorf("adb binary: %w", err)
	}
	a.client = adb.NewClient(bin)
	if _, _, err := a.client.RunVersion(ctx); err != nil {
		return fmt.Errorf("adb start check failed: %w", err)
	}
	a.engine = transfer.NewEngine(a.client)

	a.queue = queue.NewManager(2)
	a.queue.SetCallback(func(task *model.TransferTask) {
		application.Get().Event.Emit("task:changed", task)
	})

	devCtx, cancel := context.WithCancel(ctx)
	a.cancelDev = cancel
	go a.pollDevices(devCtx)

	// Auto-check for updates 3s after startup, off the critical launch path.
	// adb setup and device polling have already begun by then.
	time.AfterFunc(3*time.Second, func() { a.autoCheck(devCtx) })
	return nil
}

func (a *App) ServiceShutdown() error {
	if a.cancelDev != nil {
		a.cancelDev()
	}
	return nil
}

// pollDevices queries `adb devices` every 2s and emits a `device:changed`
// event carrying the full current list whenever the set changes.
func (a *App) pollDevices(ctx context.Context) {
	var last string
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			devs, err := a.client.ListDevices(ctx)
			if err != nil {
				continue
			}
			sig := deviceSignature(devs)
			if sig == last {
				continue
			}
			last = sig
			application.Get().Event.Emit("device:changed", map[string]any{"devices": devs})
		}
	}
}

// cacheDir returns the per-OS directory where the adb binary is extracted.
func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "AndroidFS", "platform-tools"), nil
}

// ensureAdbBinary resolves a usable adb binary:
//  1. a binary committed under build/adb/<os>-<arch>/ (if present),
//  2. otherwise the auto-downloaded platform-tools in the cache dir,
//  3. otherwise "adb" on PATH as a last resort.
//
// The download happens on first launch; subsequent launches reuse the cache.
// A download failure is non-fatal — we fall back to PATH adb so the app still
// starts (e.g. for users who installed platform-tools themselves).
func ensureAdbBinary() (string, error) {
	if p := adb.EmbeddedBinary(); p != "adb" && p != "adb.exe" {
		return p, nil
	}
	dir, err := cacheDir()
	if err == nil {
		if p, err := adb.EnsureDownloaded(dir); err == nil {
			return p, nil
		} else {
			log.Printf("auto-download adb failed (%v); falling back to system adb in PATH", err)
		}
	}
	return "adb", nil
}
