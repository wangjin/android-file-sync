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

// ensureAdbBinary returns the path to a usable adb binary. Prefer the embedded
// platform-tools binary (committed under build/adb/<os>-<arch>/), then the
// cache dir, then fall back to "adb" on PATH (system-installed).
func ensureAdbBinary() (string, error) {
	if p := adb.EmbeddedBinary(); p != "adb" && p != "adb.exe" {
		return p, nil
	}
	dir, err := cacheDir()
	if err != nil {
		return "adb", nil
	}
	name := "adb"
	if runtimeGOOS() == "windows" {
		name = "adb.exe"
	}
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	log.Println("adb binary not yet extracted; falling back to system adb in PATH")
	return name, nil
}
