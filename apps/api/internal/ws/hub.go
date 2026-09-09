// Package ws is the WebSocket push hub. Business modules publish Events to a
// tenant (every connected user of that tenant) or to one user; the hub fans
// them out to the matching connections.
//
// Wire format (server → client), one JSON object per message:
//
//	{"type":"vehicle.status","ts":"2026-09-09T10:00:00Z","data":{...}}
//
// Clients may send {"type":"ping"} and receive {"type":"pong"}.
package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Event is what modules publish.
type Event struct {
	Type string    `json:"type"`
	TS   time.Time `json:"ts"`
	Data any       `json:"data,omitempty"`
}

// Well-known event types.
const (
	EvVehicleStatus    = "vehicle.status"    // data: vstatus.VehicleStatus (tenant)
	EvTripStarted      = "trip.started"      // data: {trip_id, vehicle_id, driver_id}
	EvTripEnded        = "trip.ended"        // data: {trip_id, vehicle_id}
	EvTripEvent        = "trip.event"        // data: trip_events row (deviation/overspeed/low_soc...)
	EvApprovalUpdated  = "approval.updated"  // data: {approval_id, status} (tenant)
	EvNotification     = "notification.new"  // data: notification row (user)
	EvAlert            = "alert.new"         // phase 4
	EvDeviceOnline     = "device.online"     // data: {device_id, vehicle_id, online}
	EvChargingUpdated  = "charging.updated"  // phase 3
)

type client struct {
	userID   uuid.UUID
	tenantID uuid.UUID
	send     chan []byte
}

// Hub keeps the live connections.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	log     zerolog.Logger
}

func NewHub(log zerolog.Logger) *Hub {
	return &Hub{clients: map[*client]struct{}{}, log: log}
}

const sendBuffer = 64

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func encode(ev Event) []byte {
	if ev.TS.IsZero() {
		ev.TS = time.Now()
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return nil
	}
	return b
}

// Publish sends the event to every connection of the tenant.
func (h *Hub) Publish(tenantID uuid.UUID, ev Event) {
	if h == nil {
		return
	}
	msg := encode(ev)
	if msg == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.tenantID == tenantID {
			h.deliver(c, msg)
		}
	}
}

// PublishUser sends the event to every connection of one user.
func (h *Hub) PublishUser(userID uuid.UUID, ev Event) {
	if h == nil {
		return
	}
	msg := encode(ev)
	if msg == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.userID == userID {
			h.deliver(c, msg)
		}
	}
}

// deliver never blocks: a slow client just loses the message.
func (h *Hub) deliver(c *client, msg []byte) {
	select {
	case c.send <- msg:
	default:
		h.log.Warn().Str("user", c.userID.String()).Msg("ws client buffer full, dropping event")
	}
}

// Stats returns connection counts (for /ready or debugging).
func (h *Hub) Stats() (total int, byTenant map[uuid.UUID]int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	byTenant = map[uuid.UUID]int{}
	for c := range h.clients {
		byTenant[c.tenantID]++
	}
	return len(h.clients), byTenant
}
