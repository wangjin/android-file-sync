package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"androidfs/internal/model"
)

const (
	minMonitorIntervalMs = 200
	maxMonitorIntervalMs = 10000
)

// StartMonitor begins periodic sampling of CPU and NPU usage for the given
// device, emitting a "monitor:sample" event after each sample. A second call
// restarts the loop with the new serial/interval.
func (a *App) StartMonitor(serial string, intervalMs int) error {
	a.mu.Lock()
	if a.monitorCancel != nil {
		a.monitorCancel()
	}
	if intervalMs < minMonitorIntervalMs {
		intervalMs = 1000
	}
	if intervalMs > maxMonitorIntervalMs {
		intervalMs = maxMonitorIntervalMs
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.monitorCancel = cancel
	a.mu.Unlock()

	// Emit an initial empty frame so the frontend immediately knows the
	// monitor is live (the UI then shows "collecting…").
	a.sampleOnce(ctx, serial)

	interval := time.Duration(intervalMs) * time.Millisecond
	go a.monitorLoop(ctx, serial, interval)
	return nil
}

// StopMonitor halts the active monitor (no-op if not running).
func (a *App) StopMonitor() {
	a.mu.Lock()
	cancel := a.monitorCancel
	a.monitorCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// monitorLoop samples CPU and NPU usage on a fixed cadence until the context
// is cancelled. A per-sample timeout prevents a hung adb call from stalling
// the loop; failures produce a sample with nil metrics so the frontend can
// render gaps instead of freezing.
func (a *App) monitorLoop(ctx context.Context, serial string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	a.sampleOnce(ctx, serial)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sampleOnce(ctx, serial)
		}
	}
}

func (a *App) sampleOnce(ctx context.Context, serial string) {
	sampleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	sample := model.MonitorSample{Ts: nowMilli()}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if v, err := a.client.SampleCPU(sampleCtx, serial); err == nil {
			sample.CPU = &v
		}
	}()
	go func() {
		defer wg.Done()
		if v, err := a.client.SampleNPULoad(sampleCtx, serial); err == nil {
			sample.NPU = &v
		}
	}()
	wg.Wait()

	application.Get().Event.Emit("monitor:sample", sample)
}

// nowMilli returns the current time as unix milliseconds. Split out so tests
// can stub it out if they ever need deterministic timestamps.
func nowMilli() int64 { return time.Now().UnixMilli() }

