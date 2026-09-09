package ws

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestPublishRouting(t *testing.T) {
	h := NewHub(zerolog.Nop())
	t1, t2 := uuid.New(), uuid.New()
	u1, u2 := uuid.New(), uuid.New()
	a := &client{userID: u1, tenantID: t1, send: make(chan []byte, 4)}
	b := &client{userID: u2, tenantID: t2, send: make(chan []byte, 4)}
	h.add(a)
	h.add(b)

	h.Publish(t1, Event{Type: "x", Data: map[string]int{"n": 1}})
	if len(a.send) != 1 || len(b.send) != 0 {
		t.Fatalf("tenant routing wrong: a=%d b=%d", len(a.send), len(b.send))
	}
	var ev Event
	if err := json.Unmarshal(<-a.send, &ev); err != nil || ev.Type != "x" || ev.TS.IsZero() {
		t.Fatalf("bad payload: %v %+v", err, ev)
	}

	h.PublishUser(u2, Event{Type: "y"})
	if len(a.send) != 0 || len(b.send) != 1 {
		t.Fatalf("user routing wrong: a=%d b=%d", len(a.send), len(b.send))
	}

	// full buffer must not block
	full := &client{userID: uuid.New(), tenantID: t1, send: make(chan []byte)}
	h.add(full)
	h.Publish(t1, Event{Type: "z"})

	h.remove(a)
	if n, _ := h.Stats(); n != 2 {
		t.Fatalf("expected 2 clients after remove, got %d", n)
	}
	var nilHub *Hub
	nilHub.Publish(t1, Event{Type: "noop"}) // must not panic
}
