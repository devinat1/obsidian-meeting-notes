// services/gateway/internal/sse/hub.go
package sse

import (
	"sync"
	"sync/atomic"
)

const maxBufferSize = 100

type Event struct {
	ID   uint64
	Type string
	Data string
}

type Hub struct {
	mu          sync.RWMutex
	connections map[int]chan Event
	buffers     map[int][]Event
	counter     atomic.Uint64
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[int]chan Event),
		buffers:     make(map[int][]Event),
	}
}

func (h *Hub) Register(userID int) <-chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan Event, 64)
	h.connections[userID] = ch
	return ch
}

func (h *Hub) Unregister(userID int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.connections[userID]; ok {
		close(ch)
		delete(h.connections, userID)
	}
}

func (h *Hub) HasConnection(userID int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.connections[userID]
	return ok
}

func (h *Hub) Send(userID int, event Event) {
	event.ID = h.counter.Add(1)

	h.mu.Lock()
	buf := h.buffers[userID]
	buf = append(buf, event)
	if len(buf) > maxBufferSize {
		buf = buf[len(buf)-maxBufferSize:]
	}
	h.buffers[userID] = buf
	h.mu.Unlock()

	h.mu.RLock()
	ch, ok := h.connections[userID]
	h.mu.RUnlock()

	if ok {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *Hub) CatchUp(userID int, lastEventID uint64) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buf := h.buffers[userID]
	if len(buf) == 0 {
		return nil
	}

	var result []Event
	for _, event := range buf {
		if event.ID > lastEventID {
			result = append(result, event)
		}
	}
	return result
}
