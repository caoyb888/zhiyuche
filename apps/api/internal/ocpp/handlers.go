package ocpp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
)

// dispatch routes a CALL from a pile to the charging service.
func (s *Server) dispatch(ctx context.Context, c *conn, action string, payload json.RawMessage) (any, error) {
	now := time.Now()
	switch action {
	case "BootNotification":
		var req BootNotificationReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		if err := s.svc.Boot(ctx, c.pile.ID, req.ChargePointVendor, req.ChargePointModel); err != nil {
			return nil, err
		}
		return BootNotificationConf{Status: "Accepted", CurrentTime: FormatTime(now), Interval: int(s.svc.HeartbeatInterval().Seconds())}, nil

	case "Heartbeat":
		if err := s.svc.Heartbeat(ctx, c.pile.ID); err != nil {
			return nil, err
		}
		return HeartbeatConf{CurrentTime: FormatTime(now)}, nil

	case "StatusNotification":
		var req StatusNotificationReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		if !charging.ValidConnectorStatus(req.Status) {
			return nil, &CallErr{Code: ErrPropertyConstraintViolation, Description: "unknown status " + req.Status}
		}
		if req.ConnectorId < 0 {
			return nil, &CallErr{Code: ErrPropertyConstraintViolation, Description: "connectorId must be ≥ 0"}
		}
		var errCode *string
		if req.ErrorCode != "" {
			errCode = &req.ErrorCode
		}
		if err := s.svc.StatusNotification(ctx, c.pile.ID, req.ConnectorId, req.Status, errCode, req.Info); err != nil {
			return nil, err
		}
		return struct{}{}, nil

	case "Authorize":
		var req AuthorizeReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.IdTag) == "" {
			return nil, &CallErr{Code: ErrPropertyConstraintViolation, Description: "idTag is required"}
		}
		status, err := s.svc.Authorize(ctx, c.pile.TenantID, req.IdTag)
		if err != nil {
			return nil, err
		}
		return AuthorizeConf{IdTagInfo: IdTagInfo{Status: status}}, nil

	case "StartTransaction":
		var req StartTransactionReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.IdTag) == "" {
			return nil, &CallErr{Code: ErrPropertyConstraintViolation, Description: "idTag is required"}
		}
		ts, ok := ParseTime(req.Timestamp)
		if !ok {
			ts = now
		}
		res, err := s.svc.StartTransaction(ctx, charging.StartInput{
			PileID: c.pile.ID, ConnectorID: req.ConnectorId, IDTag: req.IdTag, MeterStart: req.MeterStart, TS: ts,
		})
		if err != nil {
			return nil, err
		}
		return StartTransactionConf{TransactionId: res.OcppTxID, IdTagInfo: IdTagInfo{Status: res.IDTagStatus}}, nil

	case "MeterValues":
		var req MeterValuesReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		samples := ParseMeterValues(req.MeterValue, now)
		if err := s.svc.MeterValues(ctx, c.pile.ID, req.ConnectorId, req.TransactionId, samples); err != nil {
			return nil, err
		}
		return struct{}{}, nil

	case "StopTransaction":
		var req StopTransactionReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		ts, ok := ParseTime(req.Timestamp)
		if !ok {
			ts = now
		}
		// transactionData carries the last samples of piles that batch them
		if len(req.TransactionData) > 0 {
			id := req.TransactionId
			if err := s.svc.MeterValues(ctx, c.pile.ID, 0, &id, ParseMeterValues(req.TransactionData, now)); err != nil {
				s.app.Log.Warn().Err(err).Msg("store transactionData failed")
			}
		}
		if err := s.svc.StopTransaction(ctx, c.pile.ID, req.TransactionId, req.MeterStop, ts, req.Reason); err != nil {
			return nil, err
		}
		conf := StopTransactionConf{}
		if req.IdTag != nil {
			conf.IdTagInfo = &IdTagInfo{Status: "Accepted"}
		}
		return conf, nil

	case "DataTransfer":
		var req DataTransferReq
		if err := decode(payload, &req); err != nil {
			return nil, err
		}
		return DataTransferConf{Status: "UnknownVendorId"}, nil
	}
	return nil, &CallErr{Code: ErrNotImplemented, Description: fmt.Sprintf("action %s is not implemented", action)}
}

// decode parses a payload; failures become FormationViolation.
func decode(payload json.RawMessage, dst any) error {
	if err := json.Unmarshal(payload, dst); err != nil {
		return &CallErr{Code: ErrFormationViolation, Description: "payload: " + err.Error()}
	}
	return nil
}
