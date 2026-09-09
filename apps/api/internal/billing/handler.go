// Package billing owns billing rules, the three-level account model
// (enterprise → department → employee), account transactions, automatic
// trip/charging deductions and monthly settlements. The calculator itself is
// internal/billing/engine. Implemented in phase 3.
package billing

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Register mounts /billing/rules, /billing/accounts, /billing/transactions, /billing/settlements.
func Register(g *gin.RouterGroup, a *app.App) {}

// NewTripHook returns the trip.CompletedHook implementation: price the trip
// with the tenant's default rule, debit the attributed account, and stamp
// trips.cost / cost_detail / billing_*.
func NewTripHook(a *app.App) func(ctx context.Context, tenantID, tripID uuid.UUID) error {
	return func(ctx context.Context, tenantID, tripID uuid.UUID) error { return errNotImplemented }
}

// NewChargingHook returns the charging.BillingHook implementation.
func NewChargingHook(a *app.App) charging.BillingHook { return &chargingHook{app: a} }

type chargingHook struct{ app *app.App }

func (h *chargingHook) PriceAndCharge(ctx context.Context, in charging.BillingInput) (*charging.BillingResult, error) {
	return nil, errNotImplemented
}

var errNotImplemented = &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"}
