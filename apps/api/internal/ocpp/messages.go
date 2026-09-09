package ocpp

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
)

// Payload shapes of the OCPP 1.6 messages this server speaks. Field names
// follow the spec (camelCase on the wire).

type IdTagInfo struct {
	Status      string  `json:"status"` // Accepted | Blocked | Expired | Invalid | ConcurrentTx
	ExpiryDate  *string `json:"expiryDate,omitempty"`
	ParentIdTag *string `json:"parentIdTag,omitempty"`
}

type BootNotificationReq struct {
	ChargePointVendor       string `json:"chargePointVendor"`
	ChargePointModel        string `json:"chargePointModel"`
	ChargePointSerialNumber string `json:"chargePointSerialNumber,omitempty"`
	FirmwareVersion         string `json:"firmwareVersion,omitempty"`
}

type BootNotificationConf struct {
	Status      string `json:"status"` // Accepted | Pending | Rejected
	CurrentTime string `json:"currentTime"`
	Interval    int    `json:"interval"` // heartbeat interval, seconds
}

type HeartbeatConf struct {
	CurrentTime string `json:"currentTime"`
}

type StatusNotificationReq struct {
	ConnectorId     int     `json:"connectorId"`
	ErrorCode       string  `json:"errorCode"`
	Info            *string `json:"info,omitempty"`
	Status          string  `json:"status"`
	Timestamp       *string `json:"timestamp,omitempty"`
	VendorId        *string `json:"vendorId,omitempty"`
	VendorErrorCode *string `json:"vendorErrorCode,omitempty"`
}

type AuthorizeReq struct {
	IdTag string `json:"idTag"`
}

type AuthorizeConf struct {
	IdTagInfo IdTagInfo `json:"idTagInfo"`
}

type StartTransactionReq struct {
	ConnectorId   int     `json:"connectorId"`
	IdTag         string  `json:"idTag"`
	MeterStart    float64 `json:"meterStart"` // Wh (integer per spec; lenient)
	ReservationId *int    `json:"reservationId,omitempty"`
	Timestamp     string  `json:"timestamp"`
}

type StartTransactionConf struct {
	IdTagInfo     IdTagInfo `json:"idTagInfo"`
	TransactionId int       `json:"transactionId"`
}

type SampledValue struct {
	Value     string `json:"value"`
	Context   string `json:"context,omitempty"`
	Format    string `json:"format,omitempty"` // Raw | SignedData
	Measurand string `json:"measurand,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Location  string `json:"location,omitempty"`
	Unit      string `json:"unit,omitempty"`
}

type MeterValueEntry struct {
	Timestamp    string         `json:"timestamp"`
	SampledValue []SampledValue `json:"sampledValue"`
}

type MeterValuesReq struct {
	ConnectorId   int               `json:"connectorId"`
	TransactionId *int              `json:"transactionId,omitempty"`
	MeterValue    []MeterValueEntry `json:"meterValue"`
}

type StopTransactionReq struct {
	IdTag           *string           `json:"idTag,omitempty"`
	MeterStop       float64           `json:"meterStop"` // Wh
	Timestamp       string            `json:"timestamp"`
	TransactionId   int               `json:"transactionId"`
	Reason          string            `json:"reason,omitempty"` // EmergencyStop | EVDisconnected | HardReset | Local | Other | PowerLoss | Reboot | Remote | SoftReset | UnlockCommand | DeAuthorized
	TransactionData []MeterValueEntry `json:"transactionData,omitempty"`
}

type StopTransactionConf struct {
	IdTagInfo *IdTagInfo `json:"idTagInfo,omitempty"`
}

type DataTransferReq struct {
	VendorId  string  `json:"vendorId"`
	MessageId *string `json:"messageId,omitempty"`
	Data      *string `json:"data,omitempty"`
}

type DataTransferConf struct {
	Status string  `json:"status"` // Accepted | Rejected | UnknownMessageId | UnknownVendorId
	Data   *string `json:"data,omitempty"`
}

// ---- central system → charge point

type RemoteStartTransactionReq struct {
	ConnectorId *int   `json:"connectorId,omitempty"`
	IdTag       string `json:"idTag"`
}

type RemoteStopTransactionReq struct {
	TransactionId int `json:"transactionId"`
}

type ResetReq struct {
	Type string `json:"type"` // Hard | Soft
}

type ChangeAvailabilityReq struct {
	ConnectorId int    `json:"connectorId"`
	Type        string `json:"type"` // Inoperative | Operative
}

type GetConfigurationReq struct {
	Key []string `json:"key,omitempty"`
}

type KeyValue struct {
	Key      string  `json:"key"`
	Readonly bool    `json:"readonly"`
	Value    *string `json:"value,omitempty"`
}

type GetConfigurationConf struct {
	ConfigurationKey []KeyValue `json:"configurationKey,omitempty"`
	UnknownKey       []string   `json:"unknownKey,omitempty"`
}

// StatusConf is the {status} answer of RemoteStart/RemoteStop/Reset/ChangeAvailability.
type StatusConf struct {
	Status string `json:"status"`
}

// ---- helpers

// Measurands this server understands.
const (
	MeasurandEnergy  = "Energy.Active.Import.Register"
	MeasurandVoltage = "Voltage"
	MeasurandCurrent = "Current.Import"
	MeasurandPower   = "Power.Active.Import"
	MeasurandSoC     = "SoC"
)

// FormatTime renders a dateTime the way OCPP expects (RFC 3339, UTC).
func FormatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ParseTime accepts the dateTime variants piles actually send; ok=false when unparsable.
func ParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ParseMeterValues converts MeterValues entries into normalised samples:
// energy → Wh, power → kW; a value without a phase wins over per-phase ones.
// Entries with an unparsable timestamp use `now`.
func ParseMeterValues(entries []MeterValueEntry, now time.Time) []charging.MeterSample {
	out := make([]charging.MeterSample, 0, len(entries))
	for _, e := range entries {
		ts, ok := ParseTime(e.Timestamp)
		if !ok {
			ts = now
		}
		s := charging.MeterSample{TS: ts}
		if raw, err := json.Marshal(e); err == nil {
			s.Raw = raw
		}
		type pick struct {
			v     float64
			phase bool
			set   bool
		}
		picks := map[string]*pick{}
		for _, sv := range e.SampledValue {
			if strings.EqualFold(sv.Format, "SignedData") {
				continue
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(sv.Value), 64)
			if err != nil {
				continue
			}
			m := sv.Measurand
			if m == "" {
				m = MeasurandEnergy
			}
			v = normalise(m, sv.Unit, v)
			hasPhase := sv.Phase != ""
			p := picks[m]
			if p == nil {
				picks[m] = &pick{v: v, phase: hasPhase, set: true}
				continue
			}
			if p.phase && !hasPhase { // prefer the aggregate reading
				p.v, p.phase = v, false
			}
		}
		if p := picks[MeasurandEnergy]; p != nil {
			s.Wh = &p.v
		}
		if p := picks[MeasurandVoltage]; p != nil {
			s.Voltage = &p.v
		}
		if p := picks[MeasurandCurrent]; p != nil {
			s.Current = &p.v
		}
		if p := picks[MeasurandPower]; p != nil {
			s.PowerKw = &p.v
		}
		if p := picks[MeasurandSoC]; p != nil {
			s.SOC = &p.v
		}
		out = append(out, s)
	}
	return out
}

// normalise converts a sampled value to the unit the store uses.
func normalise(measurand, unit string, v float64) float64 {
	switch measurand {
	case MeasurandEnergy:
		if strings.EqualFold(unit, "kWh") {
			return v * 1000
		}
		return v // Wh (default)
	case MeasurandPower:
		if strings.EqualFold(unit, "kW") {
			return v
		}
		return v / 1000 // W (default)
	case MeasurandCurrent, MeasurandVoltage, MeasurandSoC:
		return v
	}
	return v
}
