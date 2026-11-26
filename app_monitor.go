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
	interval := time.Duration(intervalMs) * time.Millisecond
	a.sampleOnce(ctx, serial, interval)

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
	a.sampleOnce(ctx, serial, interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sampleOnce(ctx, serial, interval)
		}
	}
}

// cpuSampleSleep derives the /proc/stat diff window from the sample
// interval: interval/3 clamped to [100ms, 1s] so the in-command sleep never
// starves the ticker even at the 200ms minimum interval.
func cpuSampleSleep(interval time.Duration) time.Duration {
	s := interval / 3
	if s < 100*time.Millisecond {
		s = 100 * time.Millisecond
	}
	if s > time.Second {
		s = time.Second
	}
	return s
}

func (a *App) sampleOnce(ctx context.Context, serial string, interval time.Duration) {
	sampleCtx, cancel := context.WithTimeout(ctx, 5*time.Second+cpuSampleSleep(interval))
	defer cancel()

	sample := model.MonitorSample{Ts: nowMilli()}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if v, cores, err := a.client.SampleCPUCores(sampleCtx, serial, cpuSampleSleep(interval)); err == nil {
			sample.CPU = &v
			sample.CPUCores = cores
		}
	}()
	go func() {
		defer wg.Done()
		if v, cores, err := a.client.SampleNPULoads(sampleCtx, serial); err == nil {
			sample.NPU = &v
			sample.NPUCores = cores
		}
	}()
	wg.Wait()

	application.Get().Event.Emit("monitor:sample", sample)
}

// nowMilli returns the current time as unix milliseconds. Split out so tests
// can stub it out if they ever need deterministic timestamps.
func nowMilli() int64 { return time.Now().UnixMilli() }
