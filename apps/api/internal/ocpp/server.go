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
//
// Implemented in phase 3.
package ocpp

import (
	"context"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
)

// Server accepts pile connections.
type Server struct {
	app     *app.App
	billing charging.BillingHook
}

// New creates the server; Run listens on cfg.OCPP.Addr until ctx is cancelled.
func New(a *app.App, billing charging.BillingHook) *Server { return &Server{app: a, billing: billing} }

// Run blocks until ctx is done. With an empty address it returns immediately.
func (s *Server) Run(ctx context.Context) error {
	if s.app.Cfg.OCPP.Addr == "" {
		return nil
	}
	s.app.Log.Info().Str("addr", s.app.Cfg.OCPP.Addr).Msg("ocpp server not implemented yet; listener not started")
	<-ctx.Done()
	return nil
}
