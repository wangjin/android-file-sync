package queue

import (
	"fmt"
	"sync"

	"androidfs/internal/model"
)

type Callback func(task *model.TransferTask)

// Manager is a concurrency-limited store of transfer tasks. It does not run
// tasks itself — the caller (engine) starts a task when notified it may.
type Manager struct {
	maxConcurrent int
	tasks         []*model.TransferTask
	mu            sync.RWMutex
	cb            Callback
}

func NewManager(maxConcurrent int) *Manager {
	return &Manager{maxConcurrent: maxConcurrent, tasks: make([]*model.TransferTask, 0)}
}

func (m *Manager) SetCallback(cb Callback) { m.cb = cb }

func (m *Manager) Add(task *model.TransferTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, task)
	m.notify(task)
}

func (m *Manager) Get(id string) *model.TransferTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (m *Manager) GetAll() []*model.TransferTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*model.TransferTask, len(m.tasks))
	copy(out, m.tasks)
	return out
}

func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tasks {
		if t.ID == id {
			if t.IsTerminal() {
				return fmt.Errorf("cannot cancel task in state %s", t.State)
			}
			t.State = model.StateCancelled
			m.notify(t)
			return nil
		}
	}
	return fmt.Errorf("task not found: %s", id)
}

func (m *Manager) UpdateState(id string, state model.TransferState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tasks {
		if t.ID == id {
			t.State = state
			m.notify(t)
			return
		}
	}
}

func (m *Manager) UpdateProgress(id string, bytes int64, speed float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tasks {
		if t.ID == id {
			t.Bytes = bytes
			t.Speed = speed
			m.notify(t)
			return
		}
	}
}

func (m *Manager) notify(task *model.TransferTask) {
	if m.cb != nil {
		go m.cb(task)
	}
}
