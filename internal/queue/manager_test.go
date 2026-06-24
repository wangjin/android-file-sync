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
	m.UpdateProgress("t3", 50, 1)
	task := <-got
	if task.Bytes != 50 {
		t.Fatalf("bytes=%d", task.Bytes)
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
