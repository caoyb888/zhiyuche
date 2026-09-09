package ws

import (
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	maxMsgSize = 4 * 1024
)

// Principal is the minimum the handler needs from the auth layer.
type Principal struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
}

// Handler upgrades an authenticated request. resolve returns the caller; the
// route must sit behind the auth middleware (token via ?access_token=).
func (h *Hub) Handler(allowedOrigins []string, resolve func(c *gin.Context) (Principal, bool)) gin.HandlerFunc {
	allowAll := slices.Contains(allowedOrigins, "*")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || allowAll || slices.Contains(allowedOrigins, origin)
		},
	}
	return func(c *gin.Context) {
		p, ok := resolve(c)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return // upgrader already wrote the error
		}
		cl := &client{userID: p.UserID, tenantID: p.TenantID, send: make(chan []byte, sendBuffer)}
		h.add(cl)
		go h.writeLoop(conn, cl)
		h.readLoop(conn, cl)
	}
}

func (h *Hub) readLoop(conn *websocket.Conn, cl *client) {
	defer func() {
		h.remove(cl)
		_ = conn.Close()
	}()
	conn.SetReadLimit(maxMsgSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		// application-level ping from browsers that cannot send control frames
		if string(msg) == `{"type":"ping"}` {
			h.deliver(cl, []byte(`{"type":"pong"}`))
		}
	}
}

func (h *Hub) writeLoop(conn *websocket.Conn, cl *client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	for {
		select {
		case msg, ok := <-cl.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
