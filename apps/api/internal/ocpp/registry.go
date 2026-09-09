package ocpp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
)

const (
	writeWait   = 10 * time.Second
	callTimeout = 30 * time.Second // central → pile CALL must be answered within this
)

// conn is one live pile connection. Writes are serialised; CALLs issued by the
// central system wait for their CALLRESULT/CALLERROR by UniqueId.
type conn struct {
	pile charging.PileLive
	ws   *websocket.Conn

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan *Message

	closeOnce sync.Once
	closed    chan struct{}
	lastSeen  time.Time
	seenMu    sync.Mutex
}

func newConn(p charging.PileLive, ws *websocket.Conn) *conn {
	return &conn{pile: p, ws: ws, pending: map[string]chan *Message{}, closed: make(chan struct{}), lastSeen: time.Now()}
}

func (c *conn) touch() time.Time {
	c.seenMu.Lock()
	defer c.seenMu.Unlock()
	prev := c.lastSeen
	c.lastSeen = time.Now()
	return prev
}

// write sends one text frame.
func (c *conn) write(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(websocket.TextMessage, data)
}

// call sends a CALL and waits for the answer (callTimeout or ctx).
func (c *conn) call(ctx context.Context, action string, payload any) (json.RawMessage, error) {
	uid := NewUniqueID()
	frame, err := EncodeCall(uid, action, payload)
	if err != nil {
		return nil, err
	}
	ch := make(chan *Message, 1)
	c.pendingMu.Lock()
	c.pending[uid] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, uid)
		c.pendingMu.Unlock()
	}()
	if err := c.write(frame); err != nil {
		return nil, err
	}
	timer := time.NewTimer(callTimeout)
	defer timer.Stop()
	select {
	case m := <-ch:
		if m.Type == CallError {
			return nil, &CallErr{Code: m.ErrorCode, Description: m.ErrorDescription}
		}
		return m.Payload, nil
	case <-timer.C:
		return nil, errors.New("pile did not answer within 30s")
	case <-c.closed:
		return nil, errors.New("pile disconnected")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// resolve delivers a CALLRESULT/CALLERROR to the waiting call; unknown ids are dropped.
func (c *conn) resolve(m *Message) bool {
	c.pendingMu.Lock()
	ch, ok := c.pending[m.UniqueID]
	c.pendingMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- m:
	default:
	}
	return true
}

// close ends the connection (idempotent) and wakes every waiting call.
func (c *conn) close(code int, reason string) {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.writeMu.Lock()
		_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
		c.writeMu.Unlock()
		_ = c.ws.Close()
	})
}

// registry maps pile id → live connection.
type registry struct {
	mu     sync.RWMutex
	byPile map[uuid.UUID]*conn
}

func newRegistry() *registry { return &registry{byPile: map[uuid.UUID]*conn{}} }

// add registers c and returns the connection it replaced (nil if none).
func (r *registry) add(c *conn) *conn {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.byPile[c.pile.ID]
	r.byPile[c.pile.ID] = c
	return old
}

// remove unregisters c only if it is still the current connection of its pile.
func (r *registry) remove(c *conn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byPile[c.pile.ID] != c {
		return false
	}
	delete(r.byPile, c.pile.ID)
	return true
}

func (r *registry) get(pileID uuid.UUID) *conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byPile[pileID]
}

func (r *registry) online(pileID uuid.UUID) bool { return r.get(pileID) != nil }

func (r *registry) all() []*conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*conn, 0, len(r.byPile))
	for _, c := range r.byPile {
		out = append(out, c)
	}
	return out
}

func (r *registry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byPile)
}
