package queue

import (
	"testing"

	"androidfs/internal/model"
)

func TestAddAndGet(t *testing.T) {
	m := NewManager(2)
	m.Add(&model.TransferTask{ID: "t1", State: model.StatePending})
	if len(m.GetAll()) != 1 {
		t.Fatal("expected 1 task")
	}
	if m.Get("t1") == nil {
		t.Fatal("missing t1")
	}
}

func TestCancelTerminalRejected(t *testing.T) {
	m := NewManager(2)
	m.Add(&model.TransferTask{ID: "t2", State: model.StateDone})
	if err := m.Cancel("t2"); err == nil {
		t.Fatal("expected error cancelling terminal task")
	}
}

func TestUpdateProgressNotifies(t *testing.T) {
	m := NewManager(2)
	m.Add(&model.TransferTask{ID: "t3", State: model.StateActive, Total: 100})
	got := make(chan *model.TransferTask, 1)
	m.SetCallback(func(task *model.TransferTask) { got <- task })
	m.UpdateProgress("t3", 50, 4500, 1)
	task := <-got
	if task.Bytes != 50 {
		t.Fatalf("bytes=%d", task.Bytes)
	}
	// Total must be propagated: the frontend computes pct = bytes/total, so a
	// dropped Total shows as 0% for the entire transfer.
	if task.Total != 4500 {
		t.Fatalf("total=%d want 4500", task.Total)
	}
	// The stored task carries it too (GetTasks / events read from storage).
	if m.Get("t3").Total != 4500 {
		t.Fatalf("stored total=%d want 4500", m.Get("t3").Total)
	}
}

func TestSetResult(t *testing.T) {
	m := NewManager(2)
	m.Add(&model.TransferTask{ID: "t4", State: model.StateActive})
	got := make(chan *model.TransferTask, 1)
	m.SetCallback(func(task *model.TransferTask) { got <- task })
	m.SetResult("t4", model.StateFailed, "permission denied")
	task := <-got
	if task.State != model.StateFailed {
		t.Fatalf("state=%v want failed", task.State)
	}
	if task.Error != "permission denied" {
		t.Fatalf("error=%q", task.Error)
	}
	// The stored task must carry the result too.
	if m.Get("t4").State != model.StateFailed {
		t.Fatal("stored task state not updated")
	}
}
