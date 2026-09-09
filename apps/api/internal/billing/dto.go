package billing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
)

// Account levels (accounts.level).
const (
	LevelEnterprise = engine.AttrEnterprise
	LevelDepartment = engine.AttrDepartment
	LevelEmployee   = engine.AttrEmployee
)

// Transaction types (account_transactions.type).
const (
	TxnRecharge    = "recharge"
	TxnAllocateIn  = "allocate_in"
	TxnAllocateOut = "allocate_out"
	TxnTrip        = "trip"
	TxnCharge      = "charge"
	TxnPenalty     = "penalty"
	TxnRefund      = "refund"
	TxnAdjust      = "adjust"
)

// Reference types (account_transactions.ref_type).
const (
	RefTrip       = "trip"
	RefChargeTx   = "charge_transaction"
	RefAccount    = "account"
	RefSettlement = "settlement"
)

// Trip billing states (trips.billing_status).
const (
	BillingPending = "pending"
	BillingCharged = "charged"
	BillingSkipped = "skipped"
	BillingFailed  = "failed"
)

// Settlement states.
const (
	SettlementDraft     = "draft"
	SettlementConfirmed = "confirmed"
)

// EvAccountUpdated is the WebSocket event published (to the tenant) after a
// balance change: data {account_id, level, owner_id, balance}.
const EvAccountUpdated = "account.updated"

// ---- rules ----

// Rule is the API representation of billing_rules.
type Rule struct {
	ID            uuid.UUID   `json:"id" db:"id"`
	TenantID      uuid.UUID   `json:"tenant_id" db:"tenant_id"`
	Name          string      `json:"name" db:"name"`
	Raw           []byte      `json:"-" db:"rule"`
	Doc           engine.Rule `json:"rule" db:"-"`
	IsDefault     bool        `json:"is_default" db:"is_default"`
	Enabled       bool        `json:"enabled" db:"enabled"`
	EffectiveFrom *string     `json:"effective_from" db:"effective_from"`
	EffectiveTo   *string     `json:"effective_to" db:"effective_to"`
	CreatedBy     *uuid.UUID  `json:"created_by" db:"created_by"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" db:"updated_at"`
}

// hydrate decodes the stored JSON document into Doc.
func (r *Rule) hydrate() {
	if len(r.Raw) == 0 {
		return
	}
	var d engine.Rule
	if err := json.Unmarshal(r.Raw, &d); err == nil {
		r.Doc = d
	}
}

type RuleCreateRequest struct {
	Name          string          `json:"name" binding:"required,min=1,max=64"`
	Rule          json.RawMessage `json:"rule" binding:"required"`
	EffectiveFrom *string         `json:"effective_from" binding:"omitempty,datetime=2006-01-02"`
	EffectiveTo   *string         `json:"effective_to" binding:"omitempty,datetime=2006-01-02"`
	Activate      bool            `json:"activate"`
}

// RuleUpdateRequest: effective_from / effective_to accept JSON null to clear
// (RawMessage distinguishes absent from null).
type RuleUpdateRequest struct {
	Name          *string         `json:"name" binding:"omitempty,min=1,max=64"`
	Rule          json.RawMessage `json:"rule"`
	Enabled       *bool           `json:"enabled"`
	EffectiveFrom json.RawMessage `json:"effective_from"`
	EffectiveTo   json.RawMessage `json:"effective_to"`
}

// SimulateRequest: rule_id / rule / neither (effective rule).
type SimulateRequest struct {
	RuleID *uuid.UUID      `json:"rule_id"`
	Rule   json.RawMessage `json:"rule"`
	Trip   TripInputDTO    `json:"trip" binding:"required"`
}

// TripInputDTO mirrors BillingTripInput.
type TripInputDTO struct {
	TripType        string     `json:"trip_type" binding:"omitempty,oneof=official daily"`
	StartAt         time.Time  `json:"start_at" binding:"required"`
	EndAt           time.Time  `json:"end_at" binding:"required"`
	DistanceKm      float64    `json:"distance_km"`
	EnergyKwh       float64    `json:"energy_kwh"`
	EndSOC          *float64   `json:"end_soc"`
	OverspeedEvents int        `json:"overspeed_events"`
	HarshEvents     int        `json:"harsh_events"`
	PlannedEnd      *time.Time `json:"planned_end"`
}

func (d TripInputDTO) engineInput() engine.TripInput {
	tt := d.TripType
	if tt == "" {
		tt = "official"
	}
	return engine.TripInput{
		TripType: tt, StartAt: d.StartAt, EndAt: d.EndAt, DistanceKm: d.DistanceKm, EnergyKwh: d.EnergyKwh,
		EndSOC: d.EndSOC, OverspeedEvents: d.OverspeedEvents, HarshEvents: d.HarshEvents, PlannedEnd: d.PlannedEnd,
	}
}

// ---- accounts ----

// Account is the API representation of accounts (+ owner names, month spend).
type Account struct {
	ID            uuid.UUID `json:"id" db:"id"`
	TenantID      uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Level         string    `json:"level" db:"level"`
	OwnerID       uuid.UUID `json:"owner_id" db:"owner_id"`
	OwnerName     string    `json:"owner_name" db:"owner_name"`
	OwnerSub      *string   `json:"owner_sub" db:"owner_sub"`
	Balance       float64   `json:"balance" db:"balance"`
	CreditLimit   float64   `json:"credit_limit" db:"credit_limit"`
	MonthlyBudget float64   `json:"monthly_budget" db:"monthly_budget"`
	MonthSpent    float64   `json:"month_spent" db:"month_spent"`
	Status        string    `json:"status" db:"status"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// AccountNode is a tree node (GET /billing/accounts/tree); exists=false marks
// a department/employee whose account row has not been created yet.
type AccountNode struct {
	Account
	Exists   bool           `json:"exists"`
	Children []*AccountNode `json:"children"`
}

// MyAccount is GET /billing/accounts/me.
type MyAccount struct {
	Account
	Exists       bool          `json:"exists"`
	Transactions []Transaction `json:"transactions"`
}

// Transaction is the API representation of account_transactions.
type Transaction struct {
	ID            int64      `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	AccountID     uuid.UUID  `json:"account_id" db:"account_id"`
	AccountLevel  string     `json:"account_level" db:"account_level"`
	AccountName   string     `json:"account_name" db:"account_name"`
	Type          string     `json:"type" db:"type"`
	Amount        float64    `json:"amount" db:"amount"`
	BalanceAfter  float64    `json:"balance_after" db:"balance_after"`
	RefType       *string    `json:"ref_type" db:"ref_type"`
	RefID         *string    `json:"ref_id" db:"ref_id"`
	RefNo         *string    `json:"ref_no" db:"ref_no"`
	Remark        *string    `json:"remark" db:"remark"`
	CreatedBy     *uuid.UUID `json:"created_by" db:"created_by"`
	CreatedByName *string    `json:"created_by_name" db:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

type AccountListQuery struct {
	Level    string `form:"level" binding:"omitempty,oneof=enterprise department employee"`
	Keyword  string `form:"keyword" binding:"max=64"`
	Negative bool   `form:"negative"`
}

type TxnListQuery struct {
	Level   string     `form:"level" binding:"omitempty,oneof=enterprise department employee"`
	Type    string     `form:"type" binding:"omitempty,oneof=recharge allocate_in allocate_out trip charge penalty refund adjust"`
	Keyword string     `form:"keyword" binding:"max=64"`
	From    *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	To      *time.Time `form:"to" time_format:"2006-01-02T15:04:05Z07:00"`
}

type RechargeRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Remark string  `json:"remark" binding:"max=256"`
}

type AccountUpdateRequest struct {
	CreditLimit   *float64 `json:"credit_limit" binding:"omitempty,gte=0"`
	MonthlyBudget *float64 `json:"monthly_budget" binding:"omitempty,gte=0"`
	Status        *string  `json:"status" binding:"omitempty,oneof=active frozen"`
}

type AllocateRequest struct {
	ToAccountID *uuid.UUID `json:"to_account_id"`
	ToLevel     string     `json:"to_level" binding:"omitempty,oneof=enterprise department employee"`
	ToOwnerID   *uuid.UUID `json:"to_owner_id"`
	Amount      float64    `json:"amount" binding:"required,gt=0"`
	Remark      string     `json:"remark" binding:"max=256"`
}

type AllocateResponse struct {
	From Transaction `json:"from"`
	To   Transaction `json:"to"`
}

type AdjustRequest struct {
	Amount  float64 `json:"amount" binding:"required"`
	Type    string  `json:"type" binding:"omitempty,oneof=adjust refund"`
	Remark  string  `json:"remark" binding:"required,min=1,max=256"`
	RefType string  `json:"ref_type" binding:"max=32"`
	RefID   string  `json:"ref_id" binding:"max=64"`
}

// ---- settlements ----

type Settlement struct {
	ID              uuid.UUID        `json:"id" db:"id"`
	TenantID        uuid.UUID        `json:"tenant_id" db:"tenant_id"`
	Period          string           `json:"period" db:"period"`
	DeptID          *uuid.UUID       `json:"dept_id" db:"dept_id"`
	DeptName        *string          `json:"dept_name" db:"dept_name"`
	TripCount       int              `json:"trip_count" db:"trip_count"`
	TripCost        float64          `json:"trip_cost" db:"trip_cost"`
	ChargeCount     int              `json:"charge_count" db:"charge_count"`
	ChargeCost      float64          `json:"charge_cost" db:"charge_cost"`
	Penalty         float64          `json:"penalty" db:"penalty"`
	Total           float64          `json:"total" db:"total"`
	Budget          float64          `json:"budget" db:"budget"`
	Status          string           `json:"status" db:"status"`
	GeneratedAt     time.Time        `json:"generated_at" db:"generated_at"`
	ConfirmedAt     *time.Time       `json:"confirmed_at" db:"confirmed_at"`
	ConfirmedBy     *uuid.UUID       `json:"confirmed_by" db:"confirmed_by"`
	ConfirmedByName *string          `json:"confirmed_by_name" db:"confirmed_by_name"`
	Lines           []SettlementLine `json:"lines,omitempty" db:"-"`
}

type SettlementLine struct {
	ID           int64           `json:"id" db:"id"`
	SettlementID uuid.UUID       `json:"-" db:"settlement_id"`
	Kind         string          `json:"kind" db:"kind"`
	RefID        string          `json:"ref_id" db:"ref_id"`
	RefNo        *string         `json:"ref_no" db:"ref_no"`
	UserID       *uuid.UUID      `json:"user_id" db:"user_id"`
	UserName     *string         `json:"user_name" db:"user_name"`
	VehiclePlate *string         `json:"vehicle_plate" db:"vehicle_plate"`
	OccurredAt   time.Time       `json:"occurred_at" db:"occurred_at"`
	Quantity     *float64        `json:"quantity" db:"quantity"`
	Amount       float64         `json:"amount" db:"amount"`
	Raw          []byte          `json:"-" db:"detail"`
	Detail       json.RawMessage `json:"detail" db:"-"`
}

func (l *SettlementLine) hydrate() {
	if len(l.Raw) > 0 {
		l.Detail = json.RawMessage(l.Raw)
	} else {
		l.Detail = json.RawMessage("null")
	}
}

type SettlementListQuery struct {
	Period string `form:"period" binding:"omitempty,len=7"`
	Status string `form:"status" binding:"omitempty,oneof=draft confirmed"`
}

type GenerateRequest struct {
	Period string `json:"period" binding:"required,len=7"`
}

type PeriodInfo struct {
	Period string  `json:"period" db:"period"`
	Status string  `json:"status" db:"status"`
	Total  float64 `json:"total" db:"total"`
}

type ExportQuery struct {
	Period string `form:"period" binding:"required,len=7"`
	DeptID string `form:"dept_id"`
}
