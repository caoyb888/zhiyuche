// Package ocpp is the OCPP 1.6J central system (CSMS): charge piles connect
// as WebSocket clients to ws://host:<ZY_OCPP_ADDR>/ocpp/<pile_code> with
// subprotocol "ocpp1.6". It runs inside the api process (separate listener) so
// it can call the charging service directly; cmd/ocpp stays a placeholder
// until a split is actually needed.
//
// Supported: BootNotification, Heartbeat, StatusNotification, Authorize,
// StartTransaction, MeterValues, StopTransaction, DataTransfer (ack);
// central → pile: RemoteStartTransaction, RemoteStopTransaction, Reset,
// ChangeAvailability, GetConfiguration.
package ocpp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
)

const (
	Subprotocol   = "ocpp1.6"
	pathPrefix    = "/ocpp/"
	maxFrameBytes = 256 * 1024
	sweepEvery    = 30 * time.Second
	touchEvery    = 10 * time.Second // any message counts as a heartbeat, stamped at most this often
	handleTimeout = 30 * time.Second
	closePolicy   = websocket.ClosePolicyViolation // 1008: unknown / disabled pile
)

// Server accepts pile connections and implements charging.PileGateway.
type Server struct {
	app *app.App
	svc *charging.Service
	reg *registry
}

// New creates the server and registers it as the charging module's pile gateway.
func New(a *app.App, billing charging.BillingHook) *Server {
	s := &Server{app: a, svc: charging.NewService(a, billing), reg: newRegistry()}
	charging.SetGateway(s)
	return s
}

// Run listens on cfg.OCPP.Addr until ctx is done. With an empty address it returns immediately.
func (s *Server) Run(ctx context.Context) error {
	addr := ""
	if s.app.Cfg != nil {
		addr = s.app.Cfg.OCPP.Addr
	}
	if addr == "" {
		s.app.Log.Info().Msg("ocpp server disabled (ZY_OCPP_ADDR empty)")
		return nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve accepts pile connections on ln until ctx is done (tests use it with an ephemeral port).
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go s.sweeper(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		for _, c := range s.reg.all() {
			c.close(websocket.CloseGoingAway, "server shutting down")
		}
	}()
	s.app.Log.Info().Str("addr", ln.Addr().String()).Msg("ocpp server listening")
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Addr-independent handler: GET /ocpp/{pile_code} upgrades to WebSocket.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, pathPrefix) {
		http.NotFound(w, r)
		return
	}
	code, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, pathPrefix))
	code = strings.Trim(code, "/")
	if err != nil || code == "" {
		http.Error(w, "pile code required: /ocpp/{pile_code}", http.StatusBadRequest)
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Subprotocols:    []string{Subprotocol},
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // upgrader already wrote the error
	}
	log := s.app.Log.With().Str("pile_code", code).Str("remote", r.RemoteAddr).Logger()
	if ws.Subprotocol() == "" {
		log.Warn().Msg("pile connected without the ocpp1.6 subprotocol; accepting anyway")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pile, err := s.svc.PileByCode(ctx, code)
	cancel()
	if err != nil {
		log.Error().Err(err).Msg("pile lookup failed")
		_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "lookup failed"), time.Now().Add(time.Second))
		_ = ws.Close()
		return
	}
	if pile == nil {
		log.Warn().Msg("unknown or disabled pile; closing 1008")
		_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closePolicy, "unknown pile"), time.Now().Add(time.Second))
		_ = ws.Close()
		return
	}
	c := newConn(*pile, ws)
	if old := s.reg.add(c); old != nil {
		log.Info().Msg("pile reconnected; dropping the previous connection")
		old.close(websocket.CloseNormalClosure, "replaced by a new connection")
	}
	log.Info().Str("pile_id", pile.ID.String()).Msg("pile connected")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_ = s.svc.Heartbeat(ctx, pile.ID)
	cancel()
	s.readLoop(c)
}

func (s *Server) readLoop(c *conn) {
	log := s.app.Log.With().Str("pile_code", c.pile.PileCode).Logger()
	defer func() {
		current := s.reg.remove(c)
		c.close(websocket.CloseNormalClosure, "")
		if current {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.svc.Disconnected(ctx, c.pile.ID); err != nil {
				log.Error().Err(err).Msg("mark pile offline failed")
			}
			log.Info().Msg("pile disconnected")
		}
	}()
	deadline := s.svc.OfflineAfter()
	if deadline < 90*time.Second {
		deadline = 90 * time.Second
	}
	ws := c.ws
	ws.SetReadLimit(maxFrameBytes)
	_ = ws.SetReadDeadline(time.Now().Add(deadline))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(deadline)) })
	go s.pinger(c, deadline/3)

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) && !errors.Is(err, net.ErrClosed) {
				log.Debug().Err(err).Msg("read ended")
			}
			return
		}
		_ = ws.SetReadDeadline(time.Now().Add(deadline))
		if prev := c.touch(); time.Since(prev) >= touchEvery {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.svc.Heartbeat(ctx, c.pile.ID)
			cancel()
		}
		m, err := Decode(data)
		if err != nil {
			var de *DecodeError
			if errors.As(err, &de) && de.UniqueID != "" {
				if frame, e := EncodeError(de.UniqueID, de.Code, de.Reason, nil); e == nil {
					_ = c.write(frame)
				}
			}
			log.Warn().Err(err).Str("frame", truncate(string(data), 200)).Msg("bad frame")
			continue
		}
		switch m.Type {
		case Call:
			s.handleCall(c, m)
		case CallResult, CallError:
			if !c.resolve(m) {
				log.Debug().Str("uid", m.UniqueID).Msg("answer for unknown call ignored")
			}
		}
	}
}

// pinger keeps the read deadline alive on quiet connections.
func (s *Server) pinger(c *conn, every time.Duration) {
	if every < 10*time.Second {
		every = 10 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-t.C:
			c.writeMu.Lock()
			err := c.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
			c.writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// handleCall runs the action handler and answers with CALLRESULT or CALLERROR.
func (s *Server) handleCall(c *conn, m *Message) {
	ctx, cancel := context.WithTimeout(context.Background(), handleTimeout)
	defer cancel()
	log := s.app.Log.With().Str("pile_code", c.pile.PileCode).Str("action", m.Action).Str("uid", m.UniqueID).Logger()
	payload, err := s.dispatch(ctx, c, m.Action, m.Payload)
	var frame []byte
	if err != nil {
		ce := AsCallErr(err)
		if ce.Code == ErrInternalError {
			log.Error().Err(err).Msg("ocpp handler failed")
		} else {
			log.Warn().Str("code", string(ce.Code)).Str("desc", ce.Description).Msg("ocpp call rejected")
		}
		frame, err = EncodeError(m.UniqueID, ce.Code, ce.Description, ce.Details)
	} else {
		log.Debug().Msg("ocpp call handled")
		frame, err = EncodeResult(m.UniqueID, payload)
	}
	if err != nil {
		log.Error().Err(err).Msg("encode answer failed")
		return
	}
	if err := c.write(frame); err != nil {
		log.Warn().Err(err).Msg("write answer failed")
	}
}

// sweeper marks piles without a fresh heartbeat offline and drops their connections.
func (s *Server) sweeper(ctx context.Context) {
	t := time.NewTicker(sweepEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			stale, err := s.svc.SweepStale(sctx)
			cancel()
			if err != nil {
				s.app.Log.Error().Err(err).Msg("heartbeat sweep failed")
				continue
			}
			for _, p := range stale {
				s.app.Log.Warn().Str("pile_code", p.PileCode).Msg("heartbeat timeout; pile set offline")
				if c := s.reg.get(p.ID); c != nil {
					s.reg.remove(c)
					c.close(websocket.CloseGoingAway, "heartbeat timeout")
				}
			}
		}
	}
}

// ---- charging.PileGateway

// Online reports whether the pile has a live connection.
func (s *Server) Online(pileID uuid.UUID) bool { return s.reg.online(pileID) }

// Connected returns the number of live pile connections.
func (s *Server) Connected() int { return s.reg.count() }

var errNotConnected = errors.New("pile not connected")

func (s *Server) callStatus(ctx context.Context, pileID uuid.UUID, action string, req any) (string, error) {
	c := s.reg.get(pileID)
	if c == nil {
		return "", errNotConnected
	}
	raw, err := c.call(ctx, action, req)
	if err != nil {
		return "", err
	}
	var conf StatusConf
	if err := json.Unmarshal(raw, &conf); err != nil {
		return "", err
	}
	if conf.Status == "" {
		return "Rejected", nil
	}
	return conf.Status, nil
}

// RemoteStart sends RemoteStartTransaction.
func (s *Server) RemoteStart(ctx context.Context, pileID uuid.UUID, connectorID int, idTag string) (string, error) {
	req := RemoteStartTransactionReq{IdTag: idTag}
	if connectorID > 0 {
		req.ConnectorId = &connectorID
	}
	return s.callStatus(ctx, pileID, "RemoteStartTransaction", req)
}

// RemoteStop sends RemoteStopTransaction.
func (s *Server) RemoteStop(ctx context.Context, pileID uuid.UUID, ocppTxID int) (string, error) {
	return s.callStatus(ctx, pileID, "RemoteStopTransaction", RemoteStopTransactionReq{TransactionId: ocppTxID})
}

// Reset sends Reset (type Hard | Soft).
func (s *Server) Reset(ctx context.Context, pileID uuid.UUID, typ string) (string, error) {
	return s.callStatus(ctx, pileID, "Reset", ResetReq{Type: typ})
}

// ChangeAvailability sends ChangeAvailability (type Operative | Inoperative).
func (s *Server) ChangeAvailability(ctx context.Context, pileID uuid.UUID, connectorID int, typ string) (string, error) {
	return s.callStatus(ctx, pileID, "ChangeAvailability", ChangeAvailabilityReq{ConnectorId: connectorID, Type: typ})
}

// GetConfiguration asks the pile for its configuration keys (all when keys is empty).
func (s *Server) GetConfiguration(ctx context.Context, pileID uuid.UUID, keys []string) (*GetConfigurationConf, error) {
	c := s.reg.get(pileID)
	if c == nil {
		return nil, errNotConnected
	}
	raw, err := c.call(ctx, "GetConfiguration", GetConfigurationReq{Key: keys})
	if err != nil {
		return nil, err
	}
	var conf GetConfigurationConf
	if err := json.Unmarshal(raw, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
