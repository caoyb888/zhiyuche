// Package charging owns charge piles' live state (connectors), charging
// transactions, meter values, vehicle/employee attribution, the pile-vs-BMS
// cross check and the review queue. The OCPP 1.6J server (internal/ocpp)
// feeds it; the billing module prices sessions through BillingHook.
package charging

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BillingInput is what billing needs to price and debit a charging session.
type BillingInput struct {
	TenantID  uuid.UUID
	TxID      uuid.UUID
	TxNo      string
	Kwh       float64
	UserID    *uuid.UUID
	DeptID    *uuid.UUID
	VehicleID *uuid.UUID
	StartAt   time.Time
	EndAt     time.Time
}

// BillingResult is what billing did.
type BillingResult struct {
	UnitPrice    float64
	Cost         float64
	Attribution  string    // employee | department | enterprise
	AccountID    uuid.UUID // debited account
	AccountTxnID int64
}

// BillingHook is implemented by the billing module and injected at wiring time
// (billing imports charging types; charging must not import billing).
type BillingHook interface {
	// PriceAndCharge prices the session with the tenant's default rule and
	// debits the attributed account (employee/department per rule; falls back
	// to enterprise when the attributed account cannot be resolved).
	PriceAndCharge(ctx context.Context, in BillingInput) (*BillingResult, error)
}
