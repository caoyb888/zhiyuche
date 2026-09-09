package billing

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
)

// Pure, side-effect-free rules of the module. Everything here is unit-tested
// without a database.

// target is the account a charge is attributed to.
type target struct {
	Level string
	Owner uuid.UUID
}

// resolveAttribution maps the rule's attribution level onto a concrete
// account, walking down to the enterprise when the preferred owner is
// missing: employee → department → enterprise.
func resolveAttribution(attr string, userID, deptID *uuid.UUID, tenantID uuid.UUID) target {
	switch attr {
	case LevelEmployee:
		if userID != nil && *userID != uuid.Nil {
			return target{LevelEmployee, *userID}
		}
		fallthrough
	case LevelDepartment:
		if deptID != nil && *deptID != uuid.Nil {
			return target{LevelDepartment, *deptID}
		}
	}
	return target{LevelEnterprise, tenantID}
}

// canAllocate enforces the hierarchy: enterprise → department → employee.
func canAllocate(fromLevel, toLevel string) bool {
	return (fromLevel == LevelEnterprise && toLevel == LevelDepartment) ||
		(fromLevel == LevelDepartment && toLevel == LevelEmployee)
}

// sufficient reports whether an account may fund an outgoing allocation.
func sufficient(balance, creditLimit, amount float64) bool {
	return balance+creditLimit+1e-9 >= amount
}

// overdrawn reports whether a balance breached the account's credit limit.
func overdrawn(balance, creditLimit float64) bool {
	return balance < -creditLimit-1e-9
}

// periodRange returns [start, end) of a YYYY-MM period in Asia/Shanghai.
func periodRange(period string) (time.Time, time.Time, error) {
	t, err := time.ParseInLocation("2006-01", strings.TrimSpace(period), engine.Location())
	if err != nil || len(strings.TrimSpace(period)) != 7 {
		return time.Time{}, time.Time{}, errors.New("period 格式须为 YYYY-MM")
	}
	return t, t.AddDate(0, 1, 0), nil
}

// monthBounds returns [start, end) of the month containing now (Asia/Shanghai).
func monthBounds(now time.Time) (time.Time, time.Time) {
	n := now.In(engine.Location())
	start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, engine.Location())
	return start, start.AddDate(0, 1, 0)
}

// periodOf formats a time as its YYYY-MM period in Asia/Shanghai.
func periodOf(t time.Time) string { return t.In(engine.Location()).Format("2006-01") }

// withRuleName fills rule_name from the outer name when the document has none,
// so callers may send {name, rule} without repeating the name inside.
func withRuleName(raw []byte, name string) []byte {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return raw
	}
	if s, _ := m["rule_name"].(string); strings.TrimSpace(s) == "" && strings.TrimSpace(name) != "" {
		m["rule_name"] = name
		if b, err := json.Marshal(m); err == nil {
			return b
		}
	}
	return raw
}

// accountLabel renders "员工账户（张三）" style names for notifications.
func accountLabel(level, ownerName string) string {
	switch level {
	case LevelEmployee:
		return "员工账户（" + ownerName + "）"
	case LevelDepartment:
		return "部门账户（" + ownerName + "）"
	}
	return "企业账户"
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ---- settlement aggregation ----

// tripFact is what the settlement needs from one billed trip.
type tripFact struct {
	ID         uuid.UUID
	TripNo     string
	UserID     *uuid.UUID
	UserName   *string
	DeptID     *uuid.UUID
	Plate      *string
	EndAt      time.Time
	DistanceKm *float64
	Cost       float64
	Detail     *engine.Result
}

// chargeFact is what the settlement needs from one settled charging session.
type chargeFact struct {
	ID       uuid.UUID
	TxNo     string
	UserID   *uuid.UUID
	UserName *string
	DeptID   *uuid.UUID
	Plate    *string
	EndAt    time.Time
	Kwh      *float64
	Cost     float64
}

// deptInfo is a department row (budget) for the period.
type deptInfo struct {
	ID     uuid.UUID
	Name   string
	Budget float64
}

// draftLine is a settlement_lines row to insert.
type draftLine struct {
	Kind       string
	RefID      string
	RefNo      string
	UserID     *uuid.UUID
	UserName   *string
	Plate      *string
	OccurredAt time.Time
	Quantity   *float64
	Amount     float64
	Detail     any
}

// draft is one settlements row (+ its lines) to insert.
type draft struct {
	DeptID      *uuid.UUID
	DeptName    string
	TripCount   int
	TripCost    float64
	ChargeCount int
	ChargeCost  float64
	Penalty     float64
	Total       float64
	Budget      float64
	Lines       []draftLine
}

// splitTripCost separates the fine part from a trip's cost: the cost engine
// result carries penalty separately; a trip without a detail is all trip cost.
func splitTripCost(cost float64, detail *engine.Result) (tripCost, penalty float64) {
	if detail == nil || detail.Penalty <= 0 {
		return round2(cost), 0
	}
	penalty = math.Min(detail.Penalty, cost)
	return round2(cost - penalty), round2(penalty)
}

// tripLines turns a billed trip into a trip line (+ a penalty line when fined).
func tripLines(t tripFact) (lines []draftLine, tripCost, penalty float64) {
	tripCost, penalty = splitTripCost(t.Cost, t.Detail)
	var base, fines []engine.Line
	ruleName := ""
	if t.Detail != nil {
		ruleName = t.Detail.RuleName
		for _, l := range t.Detail.Lines {
			if l.Kind == "penalty" {
				fines = append(fines, l)
			} else {
				base = append(base, l)
			}
		}
	}
	if base == nil {
		base = []engine.Line{}
	}
	lines = append(lines, draftLine{
		Kind: "trip", RefID: t.ID.String(), RefNo: t.TripNo, UserID: t.UserID, UserName: t.UserName, Plate: t.Plate,
		OccurredAt: t.EndAt, Quantity: t.DistanceKm, Amount: tripCost,
		Detail: map[string]any{"rule_name": ruleName, "lines": base},
	})
	if penalty > 0 {
		if fines == nil {
			fines = []engine.Line{}
		}
		lines = append(lines, draftLine{
			Kind: "penalty", RefID: t.ID.String(), RefNo: t.TripNo, UserID: t.UserID, UserName: t.UserName, Plate: t.Plate,
			OccurredAt: t.EndAt, Amount: penalty, Detail: map[string]any{"rule_name": ruleName, "lines": fines},
		})
	}
	return lines, tripCost, penalty
}

func chargeLine(c chargeFact) draftLine {
	return draftLine{
		Kind: "charge", RefID: c.ID.String(), RefNo: c.TxNo, UserID: c.UserID, UserName: c.UserName, Plate: c.Plate,
		OccurredAt: c.EndAt, Quantity: c.Kwh, Amount: round2(c.Cost),
	}
}

// buildSettlement aggregates the period's facts: one row per department (all
// departments of the tenant, so budgets show even without spend) plus the
// enterprise row (dept nil) which sums everything, including facts whose
// owner has no department. Lines of the enterprise row are the full detail.
func buildSettlement(trips []tripFact, charges []chargeFact, depts []deptInfo) []draft {
	byDept := map[uuid.UUID]*draft{}
	order := make([]*draft, 0, len(depts)+1)
	ent := &draft{Lines: []draftLine{}}
	for _, d := range depts {
		id := d.ID
		dr := &draft{DeptID: &id, DeptName: d.Name, Budget: round2(d.Budget), Lines: []draftLine{}}
		byDept[d.ID] = dr
		order = append(order, dr)
		ent.Budget = round2(ent.Budget + d.Budget)
	}
	apply := func(dr *draft, lines []draftLine, tripN, chargeN int, tripCost, chargeCost, penalty float64) {
		dr.Lines = append(dr.Lines, lines...)
		dr.TripCount += tripN
		dr.ChargeCount += chargeN
		dr.TripCost = round2(dr.TripCost + tripCost)
		dr.ChargeCost = round2(dr.ChargeCost + chargeCost)
		dr.Penalty = round2(dr.Penalty + penalty)
	}
	for _, t := range trips {
		lines, tc, pen := tripLines(t)
		apply(ent, lines, 1, 0, tc, 0, pen)
		if t.DeptID != nil {
			if dr, ok := byDept[*t.DeptID]; ok {
				apply(dr, lines, 1, 0, tc, 0, pen)
			}
		}
	}
	for _, c := range charges {
		line := chargeLine(c)
		apply(ent, []draftLine{line}, 0, 1, 0, line.Amount, 0)
		if c.DeptID != nil {
			if dr, ok := byDept[*c.DeptID]; ok {
				apply(dr, []draftLine{line}, 0, 1, 0, line.Amount, 0)
			}
		}
	}
	out := append([]*draft{ent}, order...)
	res := make([]draft, 0, len(out))
	for _, dr := range out {
		dr.Total = round2(dr.TripCost + dr.ChargeCost + dr.Penalty)
		sort.SliceStable(dr.Lines, func(i, j int) bool { return dr.Lines[i].OccurredAt.Before(dr.Lines[j].OccurredAt) })
		res = append(res, *dr)
	}
	return res
}

// usageRate renders spend/budget as a percentage string for the export.
func usageRate(total, budget float64) string {
	if budget <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", total/budget*100)
}

// kindText / fmt helpers for the export.
func kindText(k string) string {
	switch k {
	case "trip":
		return "行程"
	case "charge":
		return "充电"
	case "penalty":
		return "罚金"
	}
	return k
}
