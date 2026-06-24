package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"androidfs/internal/model"
)

// PushFiles uploads one or more local files/dirs to remoteDir on the device.
func (a *App) PushFiles(serial string, localPaths []string, remoteDir string) error {
	for _, lp := range localPaths {
		task := &model.TransferTask{
			ID:        uuid.NewString(),
			Direction: model.DirPush,
			State:     model.StatePending,
			FileName:  filepath.Base(lp),
			SrcPath:   lp,
			DstPath:   filepath.Join(remoteDir, filepath.Base(lp)),
			Serial:    serial,
			CreatedAt: time.Now(),
		}
		a.queue.Add(task)
		go a.runTask(task)
	}
	return nil
}

// PullFiles downloads one or more remote files/dirs to localDir on the host.
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
		a.queue.UpdateProgress(t.ID, t.Bytes, t.Speed)
	})
}
