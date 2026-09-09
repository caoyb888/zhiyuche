package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/caoyb888/zhiyuche/apps/api/internal/ocpp"
)

// ocppClient is one simulated charge point's OCPP 1.6J connection: it boots,
// heartbeats, answers the central system's CALLs through handle and
// reconnects with backoff when the connection drops.
type ocppClient struct {
	url         string
	code        string
	log         *slog.Logger
	handle      func(action string, payload json.RawMessage) (any, *ocpp.CallErr)
	onConnected func(ctx context.Context)

	mu        sync.Mutex
	ws        *websocket.Conn
	pending   map[string]chan *ocpp.Message
	connected bool
	interval  time.Duration

	wmu sync.Mutex
}

func newOCPPClient(base, code string, log *slog.Logger) *ocppClient {
	return &ocppClient{url: base + "/ocpp/" + code, code: code, log: log.With("pile", code)}
}

func (c *ocppClient) isConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// run keeps the pile connected until ctx is done.
func (c *ocppClient) run(ctx context.Context) {
	backoff := 2 * time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.log.Warn("ocpp connection lost, reconnecting", "err", err, "in", backoff)
		}
		if time.Since(start) > time.Minute {
			backoff = 2 * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// session is one connection: dial → BootNotification → heartbeats until an error.
func (c *ocppClient) session(ctx context.Context) error {
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second, Subprotocols: []string{ocpp.Subprotocol}}
	ws, _, err := d.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.ws, c.pending = ws, map[string]chan *ocpp.Message{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.connected, c.ws = false, nil
		pend := c.pending
		c.pending = nil
		c.mu.Unlock()
		_ = ws.Close()
		for _, ch := range pend { // wake waiting calls; never close (the reader may still deliver)
			select {
			case ch <- nil:
			default:
			}
		}
	}()
	readErr := make(chan error, 1)
	go func() { readErr <- c.readLoop(ws) }()

	var conf ocpp.BootNotificationConf
	if err := c.callInto(ctx, "BootNotification", ocpp.BootNotificationReq{ChargePointVendor: "智御模拟", ChargePointModel: "SIM-DC60", FirmwareVersion: "sim-1.0.0"}, &conf); err != nil {
		return fmt.Errorf("boot: %w", err)
	}
	if conf.Status != "Accepted" {
		return fmt.Errorf("boot %s", conf.Status)
	}
	interval := time.Duration(conf.Interval) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	c.mu.Lock()
	c.connected, c.interval = true, interval
	c.mu.Unlock()
	c.log.Info("ocpp connected", "url", c.url, "heartbeat", interval)
	if c.onConnected != nil {
		c.onConnected(ctx)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.wmu.Lock()
			_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "simulator stopping"), time.Now().Add(time.Second))
			c.wmu.Unlock()
			return nil
		case err := <-readErr:
			return err
		case <-ticker.C:
			if _, err := c.call(ctx, "Heartbeat", struct{}{}); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
		}
	}
}

func (c *ocppClient) readLoop(ws *websocket.Conn) error {
	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		m, err := ocpp.Decode(data)
		if err != nil {
			c.log.Warn("bad frame from central system", "err", err)
			continue
		}
		switch m.Type {
		case ocpp.Call:
			go c.serve(m)
		default:
			c.mu.Lock()
			ch := c.pending[m.UniqueID]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- m:
				default:
				}
			}
		}
	}
}

// serve answers a CALL from the central system.
func (c *ocppClient) serve(m *ocpp.Message) {
	var frame []byte
	var err error
	if c.handle == nil {
		frame, err = ocpp.EncodeError(m.UniqueID, ocpp.ErrNotImplemented, "", nil)
	} else if payload, cerr := c.handle(m.Action, m.Payload); cerr != nil {
		frame, err = ocpp.EncodeError(m.UniqueID, cerr.Code, cerr.Description, cerr.Details)
	} else {
		frame, err = ocpp.EncodeResult(m.UniqueID, payload)
	}
	if err != nil {
		c.log.Warn("encode answer failed", "err", err)
		return
	}
	if err := c.write(frame); err != nil {
		c.log.Warn("answer central system failed", "action", m.Action, "err", err)
	}
}

func (c *ocppClient) write(frame []byte) error {
	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()
	if ws == nil {
		return errors.New("not connected")
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ws.WriteMessage(websocket.TextMessage, frame)
}

// call sends a CALL and waits (≤ 30 s) for the answer payload.
func (c *ocppClient) call(ctx context.Context, action string, payload any) (json.RawMessage, error) {
	uid := ocpp.NewUniqueID()
	frame, err := ocpp.EncodeCall(uid, action, payload)
	if err != nil {
		return nil, err
	}
	ch := make(chan *ocpp.Message, 1)
	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return nil, errors.New("not connected")
	}
	c.pending[uid] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.pending != nil {
			delete(c.pending, uid)
		}
		c.mu.Unlock()
	}()
	if err := c.write(frame); err != nil {
		return nil, err
	}
	select {
	case m := <-ch:
		if m == nil {
			return nil, errors.New("connection closed")
		}
		if m.Type == ocpp.CallError {
			return nil, fmt.Errorf("%s: %s", m.ErrorCode, m.ErrorDescription)
		}
		return m.Payload, nil
	case <-time.After(30 * time.Second):
		return nil, errors.New("no answer within 30s")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *ocppClient) callInto(ctx context.Context, action string, payload, dst any) error {
	raw, err := c.call(ctx, action, payload)
	if err != nil {
		return err
	}
	if dst == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}
