package main

import (
	"context"
	"path"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"androidfs/internal/model"
)

// PushFiles uploads one or more local files/dirs to remoteDir on the device.
// Device paths must use "/" as separator on every host, so the destination is
// joined with path.Join (posix) regardless of the host OS.
func (a *App) PushFiles(serial string, localPaths []string, remoteDir string) error {
	for _, lp := range localPaths {
		task := &model.TransferTask{
			ID:        uuid.NewString(),
			Direction: model.DirPush,
			State:     model.StatePending,
			FileName:  filepath.Base(lp),
			SrcPath:   lp,
			DstPath:   path.Join(remoteDir, filepath.Base(lp)),
			Serial:    serial,
			CreatedAt: time.Now(),
		}
		a.queue.Add(task)
		go a.runTask(task)
	}
	return nil
}

// PullFiles downloads one or more remote files/dirs to localDir on the host.
// The destination is a host path, so filepath.Join (host separator) is correct.
func (a *App) PullFiles(serial string, remotePaths []string, localDir string) error {
	for _, rp := range remotePaths {
		task := &model.TransferTask{
			ID:        uuid.NewString(),
			Direction: model.DirPull,
			State:     model.StatePending,
			FileName:  filepath.Base(rp),
			SrcPath:   rp,
			DstPath:   filepath.Join(localDir, filepath.Base(rp)),
			Serial:    serial,
			CreatedAt: time.Now(),
		}
		a.queue.Add(task)
		go a.runTask(task)
	}
	return nil
}

// GetTasks returns the current transfer queue.
func (a *App) GetTasks() []*model.TransferTask { return a.queue.GetAll() }

// CancelTask cancels a queued or active task.
func (a *App) CancelTask(id string) error { return a.queue.Cancel(id) }

func (a *App) runTask(task *model.TransferTask) {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.queue.UpdateState(task.ID, model.StateActive)
	a.engine.Run(ctx, task, func(t *model.TransferTask) {
		a.queue.UpdateProgress(t.ID, t.Bytes, t.Total, t.Speed)
	})
	// engine.Run mutates task.State/Task.Error in place; propagate the final
	// state (done/failed/cancelled) and any error back into the queue so the
	// frontend sees the outcome via the task:changed event.
	a.queue.SetResult(task.ID, task.State, task.Error)
	// On successful completion, tell the frontend which pane changed so it can
	// re-list it. Pull writes to the host (local pane), push writes to the
	// device. We only fire on success — a failed transfer changed nothing.
	if task.State == model.StateDone {
		side := "device"
		if task.Direction == model.DirPull {
			side = "local"
		}
		application.Get().Event.Emit("task:done", map[string]any{
			"side":      side,
			"direction": task.Direction.String(),
		})
	}
}
