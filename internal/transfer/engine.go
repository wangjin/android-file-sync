package transfer

import (
	"context"
	"time"

	"androidfs/internal/adb"
	"androidfs/internal/model"
)

// Engine executes a single TransferTask against an adb client. Concurrency
// limiting and lifecycle are owned by the queue; the engine just runs one task.
type Engine struct {
	client *adb.AdbClient
}

func NewEngine(client *adb.AdbClient) *Engine {
	return &Engine{client: client}
}

// Run executes the task to completion. Progress is reported via onProgress.
// Returns the final error (nil on success).
func (e *Engine) Run(ctx context.Context, task *model.TransferTask, onProgress func(*model.TransferTask)) error {
	progress := func(bytes, total, rate int64) {
		task.Bytes = bytes
		task.Total = total
		if rate > 0 {
			task.Speed = float64(rate)
		}
		if onProgress != nil {
			onProgress(task)
		}
	}
	task.State = model.StateActive
	if onProgress != nil {
		onProgress(task)
	}
	start := time.Now()

	var err error
	if task.Direction == model.DirPush {
		err = e.client.Push(ctx, task.Serial, task.SrcPath, task.DstPath, progress)
	} else {
		err = e.client.Pull(ctx, task.Serial, task.SrcPath, task.DstPath, progress)
	}

	if ctx.Err() != nil {
		task.State = model.StateCancelled
	} else if err != nil {
		task.State = model.StateFailed
		task.Error = err.Error()
	} else {
		task.State = model.StateDone
		elapsed := time.Since(start).Seconds()
		if elapsed > 0 {
			task.Speed = float64(task.Bytes) / elapsed
		}
	}
	now := time.Now()
	task.DoneAt = &now
	if onProgress != nil {
		onProgress(task)
	}
	return err
}
