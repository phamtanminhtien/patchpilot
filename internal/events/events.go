package events

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Event struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"createdAt"`
	Payload     any       `json:"payload"`
}

type Hub struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: map[chan Event]struct{}{}}
}

func (h *Hub) Publish(event Event) Event {
	if event.ID == "" {
		event.ID = newEventID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	return event
}

func (h *Hub) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		close(ch)
		h.mu.Unlock()
	}
}

func newEventID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "evt_fallback"
	}
	return "evt_" + hex.EncodeToString(random[:])
}
