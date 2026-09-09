package ocpp

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// OCPP-J 1.6 wire format (RPC framework, section 4):
//
//	CALL       [2, "<UniqueId>", "<Action>", {<Payload>}]
//	CALLRESULT [3, "<UniqueId>", {<Payload>}]
//	CALLERROR  [4, "<UniqueId>", "<errorCode>", "<errorDescription>", {<errorDetails>}]

// MessageType is the first array element.
type MessageType int

const (
	Call       MessageType = 2
	CallResult MessageType = 3
	CallError  MessageType = 4
)

// ErrorCode is an OCPP-J error code (section 4.2.3).
type ErrorCode string

const (
	ErrNotImplemented               ErrorCode = "NotImplemented"
	ErrNotSupported                 ErrorCode = "NotSupported"
	ErrInternalError                ErrorCode = "InternalError"
	ErrProtocolError                ErrorCode = "ProtocolError"
	ErrSecurityError                ErrorCode = "SecurityError"
	ErrFormationViolation           ErrorCode = "FormationViolation"
	ErrPropertyConstraintViolation  ErrorCode = "PropertyConstraintViolation"
	ErrOccurenceConstraintViolation ErrorCode = "OccurenceConstraintViolation"
	ErrTypeConstraintViolation      ErrorCode = "TypeConstraintViolation"
	ErrGenericError                 ErrorCode = "GenericError"
)

// Message is a decoded frame.
type Message struct {
	Type     MessageType
	UniqueID string
	Action   string          // CALL only
	Payload  json.RawMessage // CALL / CALLRESULT payload, CALLERROR details
	// CALLERROR only
	ErrorCode        ErrorCode
	ErrorDescription string
}

// DecodeError is returned by Decode; Code says which CALLERROR the peer
// deserves and UniqueID (when it could be read) lets the caller address it.
type DecodeError struct {
	Code     ErrorCode
	UniqueID string
	Reason   string
}

func (e *DecodeError) Error() string { return string(e.Code) + ": " + e.Reason }

// CallErr is what an action handler returns to answer with a CALLERROR.
type CallErr struct {
	Code        ErrorCode
	Description string
	Details     any
}

func (e *CallErr) Error() string { return string(e.Code) + ": " + e.Description }

// AsCallErr converts any handler error into a CallErr (InternalError by default).
func AsCallErr(err error) *CallErr {
	var ce *CallErr
	if errors.As(err, &ce) {
		return ce
	}
	return &CallErr{Code: ErrInternalError, Description: err.Error()}
}

// Decode parses one frame.
func Decode(data []byte) (*Message, error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return nil, &DecodeError{Code: ErrFormationViolation, Reason: "message is not a JSON array"}
	}
	if len(parts) < 3 {
		return nil, &DecodeError{Code: ErrFormationViolation, Reason: "message needs at least 3 elements"}
	}
	var typ int
	if err := json.Unmarshal(parts[0], &typ); err != nil {
		return nil, &DecodeError{Code: ErrFormationViolation, Reason: "MessageTypeId must be an integer"}
	}
	var uid string
	if err := json.Unmarshal(parts[1], &uid); err != nil || uid == "" {
		return nil, &DecodeError{Code: ErrFormationViolation, Reason: "UniqueId must be a non-empty string"}
	}
	m := &Message{Type: MessageType(typ), UniqueID: uid}
	switch m.Type {
	case Call:
		if len(parts) != 4 {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "CALL needs 4 elements"}
		}
		if err := json.Unmarshal(parts[2], &m.Action); err != nil || m.Action == "" {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "Action must be a non-empty string"}
		}
		if !isObject(parts[3]) {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "Payload must be a JSON object"}
		}
		m.Payload = parts[3]
	case CallResult:
		if len(parts) != 3 {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "CALLRESULT needs 3 elements"}
		}
		if !isObject(parts[2]) {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "Payload must be a JSON object"}
		}
		m.Payload = parts[2]
	case CallError:
		if len(parts) != 5 {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "CALLERROR needs 5 elements"}
		}
		var code string
		if err := json.Unmarshal(parts[2], &code); err != nil {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "errorCode must be a string"}
		}
		m.ErrorCode = ErrorCode(code)
		if err := json.Unmarshal(parts[3], &m.ErrorDescription); err != nil {
			return nil, &DecodeError{Code: ErrFormationViolation, UniqueID: uid, Reason: "errorDescription must be a string"}
		}
		m.Payload = parts[4]
	default:
		return nil, &DecodeError{Code: ErrProtocolError, UniqueID: uid, Reason: fmt.Sprintf("unknown MessageTypeId %d", typ)}
	}
	return m, nil
}

func isObject(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		}
		return b == '{'
	}
	return false
}

// EncodeCall builds a CALL frame.
func EncodeCall(uid, action string, payload any) ([]byte, error) {
	if payload == nil {
		payload = struct{}{}
	}
	return json.Marshal([]any{Call, uid, action, payload})
}

// EncodeResult builds a CALLRESULT frame.
func EncodeResult(uid string, payload any) ([]byte, error) {
	if payload == nil {
		payload = struct{}{}
	}
	return json.Marshal([]any{CallResult, uid, payload})
}

// EncodeError builds a CALLERROR frame.
func EncodeError(uid string, code ErrorCode, description string, details any) ([]byte, error) {
	if details == nil {
		details = struct{}{}
	}
	return json.Marshal([]any{CallError, uid, string(code), description, details})
}

// NewUniqueID makes a message id (max 36 chars per spec: a UUID fits exactly).
func NewUniqueID() string { return uuid.NewString() }
