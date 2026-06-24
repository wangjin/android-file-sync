package model

import "testing"

func TestTransferTaskIsTerminal(t *testing.T) {
	cases := []struct {
		state TransferState
		want  bool
	}{
		{StatePending, false},
		{StateActive, false},
		{StateDone, true},
		{StateFailed, true},
		{StateCancelled, true},
	}
	for _, c := range cases {
		task := &TransferTask{State: c.state}
		if got := task.IsTerminal(); got != c.want {
			t.Errorf("state %s: IsTerminal=%v want %v", c.state, got, c.want)
		}
	}
}

func TestTransferTaskProgress(t *testing.T) {
	task := &TransferTask{Total: 200, Bytes: 50}
	if got := task.Progress(); got != 0.25 {
		t.Fatalf("Progress=%v want 0.25", got)
	}
	zero := &TransferTask{Total: 0}
	if got := zero.Progress(); got != 0 {
		t.Fatalf("zero total Progress=%v want 0", got)
	}
}

func TestTransferStateString(t *testing.T) {
	if StateActive.String() != "active" {
		t.Fatalf("got %q", StateActive.String())
	}
}
