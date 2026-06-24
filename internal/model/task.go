package model

import "time"

type TransferState int

const (
	StatePending TransferState = iota
	StateActive
	StateDone
	StateFailed
	StateCancelled
)

func (s TransferState) String() string {
	names := []string{"pending", "active", "done", "failed", "cancelled"}
	if int(s) >= 0 && int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

type TransferDirection int

const (
	DirPush TransferDirection = iota
	DirPull
)

func (d TransferDirection) String() string {
	if d == DirPush {
		return "push"
	}
	return "pull"
}

// TransferTask describes one push or pull operation, tracked by the queue.
type TransferTask struct {
	ID        string            `json:"id"`
	Direction TransferDirection `json:"direction"`
	State     TransferState     `json:"state"`
	FileName  string            `json:"file_name"`
	SrcPath   string            `json:"-"`
	DstPath   string            `json:"-"`
	Serial    string            `json:"-"`
	Total     int64             `json:"total"`
	Bytes     int64             `json:"bytes"`
	Speed     float64           `json:"speed"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	DoneAt    *time.Time        `json:"done_at,omitempty"`
}

func (t *TransferTask) IsTerminal() bool {
	return t.State == StateDone || t.State == StateFailed || t.State == StateCancelled
}

func (t *TransferTask) Progress() float64 {
	if t.Total == 0 {
		return 0
	}
	return float64(t.Bytes) / float64(t.Total)
}
