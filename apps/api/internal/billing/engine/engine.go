// Package engine is the pure, side-effect-free billing calculator.
//
// A Rule is the JSON document stored in billing_rules.rule (the structure
// follows 技术方案 V1.1 §2 "可编程计费引擎"). ComputeTrip turns a finished
// trip into an itemised bill; ChargingCost prices a charging session.
package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Attribution values: which account pays.
const (
	AttrEmployee   = "employee"
	AttrDepartment = "department"
	AttrEnterprise = "enterprise"
)

// Penalty types.
const (
	PenaltyOverspeed         = "overspeed"              // per overspeed event
	PenaltyNotChargingReturn = "not_charging_on_return" // end SOC below minimum
	PenaltyHarshDriving      = "harsh_driving"          // per harsh accel/brake event
	PenaltyLateReturn        = "late_return"            // returned after planned end
)

type BaseRate struct {
	PerKm    float64 `json:"per_km"`
	PerHour  float64 `json:"per_hour"`
	DailyCap float64 `json:"daily_cap"` // 0 = no cap; applies per 24h of trip duration
}

type LowBatterySurcharge struct {
	ThresholdSOC float64 `json:"threshold_soc"` // surcharge when end SOC < threshold
	SurchargePct float64 `json:"surcharge_pct"` // % of the base amount
}

type EVSpecific struct {
	ElectricityCostPerKwh    float64             `json:"electricity_cost_per_kwh"`
	IncludeElectricityInTrip bool                `json:"include_electricity_in_trip"` // add energy × price to the trip bill
	ChargingAttribution      string              `json:"charging_attribution"`        // who pays charging sessions
	LowBatterySurcharge      LowBatterySurcharge `json:"low_battery_surcharge"`
}

type TimeMultiplier struct {
	Name   string  `json:"name"`
	Range  string  `json:"range"` // "HH:MM-HH:MM", may cross midnight; matched against trip start (Asia/Shanghai)
	Factor float64 `json:"factor"`
}

type PenaltyRule struct {
	Type           string  `json:"type"`
	ThresholdKmh   float64 `json:"threshold_kmh,omitempty"`    // overspeed (informational; events are counted upstream)
	FinePerEvent   float64 `json:"fine_per_event,omitempty"`   // overspeed / harsh_driving
	MinSOCRequired float64 `json:"min_soc_required,omitempty"` // not_charging_on_return
	Fine           float64 `json:"fine,omitempty"`             // not_charging_on_return / late_return (flat)
	FinePerHour    float64 `json:"fine_per_hour,omitempty"`    // late_return
	MaxFine        float64 `json:"max_fine,omitempty"`         // cap per trip for this rule (0 = none)
}

type TripAttribution struct {
	Official string `json:"official"` // department | employee | enterprise
	Daily    string `json:"daily"`
}

// Rule is the programmable billing rule.
type Rule struct {
	Name            string           `json:"rule_name"`
	BaseRate        BaseRate         `json:"base_rate"`
	EVSpecific      EVSpecific       `json:"ev_specific"`
	TimeMultipliers []TimeMultiplier `json:"time_multipliers"`
	PenaltyRules    []PenaltyRule    `json:"penalty_rules"`
	TripAttribution TripAttribution  `json:"trip_attribution"`
}

// Default is the 纯电车队标准套餐 from the V1.1 proposal.
func Default() Rule {
	return Rule{
		Name:     "纯电车队标准套餐",
		BaseRate: BaseRate{PerKm: 0.60, PerHour: 5.00, DailyCap: 80.00},
		EVSpecific: EVSpecific{
			ElectricityCostPerKwh:    0.65,
			IncludeElectricityInTrip: true,
			ChargingAttribution:      AttrDepartment,
			LowBatterySurcharge:      LowBatterySurcharge{ThresholdSOC: 20, SurchargePct: 10},
		},
		TimeMultipliers: []TimeMultiplier{
			{Name: "早高峰", Range: "07:30-09:00", Factor: 1.3},
			{Name: "晚高峰", Range: "17:30-19:00", Factor: 1.3},
			{Name: "深夜", Range: "23:00-06:00", Factor: 0.7},
		},
		PenaltyRules: []PenaltyRule{
			{Type: PenaltyOverspeed, ThresholdKmh: 80, FinePerEvent: 5.0, MaxFine: 50},
			{Type: PenaltyNotChargingReturn, MinSOCRequired: 30, Fine: 10.0},
		},
		TripAttribution: TripAttribution{Official: AttrDepartment, Daily: AttrEmployee},
	}
}

// Parse decodes and validates a rule document.
func Parse(raw []byte) (Rule, error) {
	var r Rule
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("规则 JSON 无法解析: %w", err)
	}
	r.applyDefaults()
	return r, r.Validate()
}

func (r *Rule) applyDefaults() {
	if r.TripAttribution.Official == "" {
		r.TripAttribution.Official = AttrDepartment
	}
	if r.TripAttribution.Daily == "" {
		r.TripAttribution.Daily = AttrEmployee
	}
	if r.EVSpecific.ChargingAttribution == "" {
		r.EVSpecific.ChargingAttribution = AttrDepartment
	}
}

func validAttr(a string) bool { return a == AttrEmployee || a == AttrDepartment || a == AttrEnterprise }

// Validate reports the first problem with the rule.
func (r Rule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("rule_name 不能为空")
	}
	b := r.BaseRate
	if b.PerKm < 0 || b.PerHour < 0 || b.DailyCap < 0 {
		return errors.New("base_rate 不能为负")
	}
	if b.PerKm == 0 && b.PerHour == 0 {
		return errors.New("base_rate 的 per_km 与 per_hour 不能同时为 0")
	}
	ev := r.EVSpecific
	if ev.ElectricityCostPerKwh < 0 {
		return errors.New("electricity_cost_per_kwh 不能为负")
	}
	if !validAttr(ev.ChargingAttribution) {
		return errors.New("charging_attribution 须为 employee/department/enterprise")
	}
	if ev.LowBatterySurcharge.ThresholdSOC < 0 || ev.LowBatterySurcharge.ThresholdSOC > 100 || ev.LowBatterySurcharge.SurchargePct < 0 {
		return errors.New("low_battery_surcharge 取值不合法")
	}
	for i, m := range r.TimeMultipliers {
		if m.Factor <= 0 {
			return fmt.Errorf("time_multipliers[%d].factor 必须大于 0", i)
		}
		if _, _, err := parseRange(m.Range); err != nil {
			return fmt.Errorf("time_multipliers[%d].range: %w", i, err)
		}
	}
	for i, p := range r.PenaltyRules {
		switch p.Type {
		case PenaltyOverspeed, PenaltyHarshDriving:
			if p.FinePerEvent < 0 {
				return fmt.Errorf("penalty_rules[%d].fine_per_event 不能为负", i)
			}
		case PenaltyNotChargingReturn:
			if p.MinSOCRequired < 0 || p.MinSOCRequired > 100 || p.Fine < 0 {
				return fmt.Errorf("penalty_rules[%d] 取值不合法", i)
			}
		case PenaltyLateReturn:
			if p.Fine < 0 || p.FinePerHour < 0 {
				return fmt.Errorf("penalty_rules[%d] 取值不合法", i)
			}
		default:
			return fmt.Errorf("penalty_rules[%d].type 未知: %q", i, p.Type)
		}
		if p.MaxFine < 0 {
			return fmt.Errorf("penalty_rules[%d].max_fine 不能为负", i)
		}
	}
	if !validAttr(r.TripAttribution.Official) || !validAttr(r.TripAttribution.Daily) {
		return errors.New("trip_attribution 须为 employee/department/enterprise")
	}
	return nil
}

// ---- computation ----

// TripInput is what the engine needs from a finished trip.
type TripInput struct {
	TripType        string // official | daily
	StartAt, EndAt  time.Time
	DistanceKm      float64
	EnergyKwh       float64
	EndSOC          *float64
	OverspeedEvents int
	HarshEvents     int
	PlannedEnd      *time.Time
}

// Line is one itemised row of the bill.
type Line struct {
	Item      string  `json:"item"`       // 里程费 / 时长费 / 电费 / 时段系数 / 低电附加 / 超速罚金 ...
	Kind      string  `json:"kind"`       // base | multiplier | cap | surcharge | electricity | penalty
	Qty       float64 `json:"qty"`        // km / h / kWh / 次
	Unit      string  `json:"unit"`       // km / 小时 / kWh / 次 / %
	UnitPrice float64 `json:"unit_price"` // 元/单位（系数行为 factor）
	Amount    float64 `json:"amount"`     // 元，罚金为正数单列，减免为负
	Note      string  `json:"note,omitempty"`
}

// Result is the priced trip.
type Result struct {
	RuleName    string  `json:"rule_name"`
	Lines       []Line  `json:"lines"`
	Base        float64 `json:"base"`        // 里程费 + 时长费（未乘系数）
	Multiplier  float64 `json:"multiplier"`  // 命中的时段系数（无则 1）
	CapApplied  bool    `json:"cap_applied"` // 是否触发日封顶
	Surcharge   float64 `json:"surcharge"`
	Electricity float64 `json:"electricity"`
	Penalty     float64 `json:"penalty"`
	Total       float64 `json:"total"`
	Attribution string  `json:"attribution"` // 扣费账户级别
}

var shanghai = mustLoad("Asia/Shanghai")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// ComputeTrip prices a finished trip. It never returns an error for odd
// inputs; negative or missing values are treated as zero.
func ComputeTrip(r Rule, in TripInput) Result {
	r.applyDefaults()
	res := Result{RuleName: r.Name, Multiplier: 1}
	dist := math.Max(in.DistanceKm, 0)
	dur := in.EndAt.Sub(in.StartAt)
	if dur < 0 {
		dur = 0
	}
	hours := dur.Hours()

	kmFee := round2(dist * r.BaseRate.PerKm)
	hourFee := round2(hours * r.BaseRate.PerHour)
	if r.BaseRate.PerKm > 0 {
		res.Lines = append(res.Lines, Line{Item: "里程费", Kind: "base", Qty: round1(dist), Unit: "km", UnitPrice: r.BaseRate.PerKm, Amount: kmFee})
	}
	if r.BaseRate.PerHour > 0 {
		res.Lines = append(res.Lines, Line{Item: "时长费", Kind: "base", Qty: round2(hours), Unit: "小时", UnitPrice: r.BaseRate.PerHour, Amount: hourFee})
	}
	res.Base = round2(kmFee + hourFee)
	amount := res.Base

	// time multiplier by trip start (local time)
	if m, ok := matchMultiplier(r.TimeMultipliers, in.StartAt.In(shanghai)); ok && m.Factor != 1 {
		delta := round2(amount*m.Factor - amount)
		res.Multiplier = m.Factor
		res.Lines = append(res.Lines, Line{Item: "时段系数·" + m.Name, Kind: "multiplier", Qty: 1, Unit: "次", UnitPrice: m.Factor, Amount: delta, Note: m.Range})
		amount = round2(amount + delta)
	}

	// daily cap (per started 24h)
	if r.BaseRate.DailyCap > 0 {
		days := math.Max(1, math.Ceil(hours/24))
		cap := round2(r.BaseRate.DailyCap * days)
		if amount > cap {
			res.CapApplied = true
			res.Lines = append(res.Lines, Line{Item: "日封顶减免", Kind: "cap", Qty: days, Unit: "天", UnitPrice: r.BaseRate.DailyCap, Amount: round2(cap - amount)})
			amount = cap
		}
	}

	// low battery surcharge
	lb := r.EVSpecific.LowBatterySurcharge
	if in.EndSOC != nil && lb.SurchargePct > 0 && *in.EndSOC < lb.ThresholdSOC {
		res.Surcharge = round2(amount * lb.SurchargePct / 100)
		res.Lines = append(res.Lines, Line{Item: "低电量还车附加", Kind: "surcharge", Qty: lb.SurchargePct, Unit: "%", UnitPrice: amount, Amount: res.Surcharge,
			Note: fmt.Sprintf("还车 SOC %.0f%% < %.0f%%", *in.EndSOC, lb.ThresholdSOC)})
	}

	// electricity
	if r.EVSpecific.IncludeElectricityInTrip && r.EVSpecific.ElectricityCostPerKwh > 0 && in.EnergyKwh > 0 {
		res.Electricity = round2(in.EnergyKwh * r.EVSpecific.ElectricityCostPerKwh)
		res.Lines = append(res.Lines, Line{Item: "电费", Kind: "electricity", Qty: round2(in.EnergyKwh), Unit: "kWh", UnitPrice: r.EVSpecific.ElectricityCostPerKwh, Amount: res.Electricity})
	}

	// penalties
	for _, p := range r.PenaltyRules {
		var fine float64
		var line Line
		switch p.Type {
		case PenaltyOverspeed:
			if in.OverspeedEvents <= 0 || p.FinePerEvent <= 0 {
				continue
			}
			fine = float64(in.OverspeedEvents) * p.FinePerEvent
			line = Line{Item: "超速罚金", Qty: float64(in.OverspeedEvents), Unit: "次", UnitPrice: p.FinePerEvent, Note: fmt.Sprintf("超过 %.0f km/h", p.ThresholdKmh)}
		case PenaltyHarshDriving:
			if in.HarshEvents <= 0 || p.FinePerEvent <= 0 {
				continue
			}
			fine = float64(in.HarshEvents) * p.FinePerEvent
			line = Line{Item: "急加减速罚金", Qty: float64(in.HarshEvents), Unit: "次", UnitPrice: p.FinePerEvent}
		case PenaltyNotChargingReturn:
			if in.EndSOC == nil || p.Fine <= 0 || *in.EndSOC >= p.MinSOCRequired {
				continue
			}
			fine = p.Fine
			line = Line{Item: "低电未充电还车罚金", Qty: 1, Unit: "次", UnitPrice: p.Fine, Note: fmt.Sprintf("还车 SOC %.0f%% < %.0f%%", *in.EndSOC, p.MinSOCRequired)}
		case PenaltyLateReturn:
			if in.PlannedEnd == nil || !in.EndAt.After(*in.PlannedEnd) {
				continue
			}
			lateH := in.EndAt.Sub(*in.PlannedEnd).Hours()
			fine = p.Fine + math.Ceil(lateH)*p.FinePerHour
			if fine <= 0 {
				continue
			}
			line = Line{Item: "超时还车罚金", Qty: math.Ceil(lateH), Unit: "小时", UnitPrice: p.FinePerHour, Note: fmt.Sprintf("超出计划 %.1f 小时", lateH)}
		default:
			continue
		}
		if p.MaxFine > 0 && fine > p.MaxFine {
			fine = p.MaxFine
			line.Note = strings.TrimSpace(line.Note + " 已按上限计")
		}
		fine = round2(fine)
		line.Kind = "penalty"
		line.Amount = fine
		res.Penalty = round2(res.Penalty + fine)
		res.Lines = append(res.Lines, line)
	}

	res.Total = round2(amount + res.Surcharge + res.Electricity + res.Penalty)
	if in.TripType == "daily" {
		res.Attribution = r.TripAttribution.Daily
	} else {
		res.Attribution = r.TripAttribution.Official
	}
	if res.Lines == nil {
		res.Lines = []Line{}
	}
	return res
}

// ChargingCost prices a charging session (kWh from the pile meter).
func ChargingCost(r Rule, kwh float64) (unitPrice, cost float64) {
	unitPrice = r.EVSpecific.ElectricityCostPerKwh
	return unitPrice, round2(math.Max(kwh, 0) * unitPrice)
}

// ---- helpers ----

func matchMultiplier(ms []TimeMultiplier, t time.Time) (TimeMultiplier, bool) {
	minute := t.Hour()*60 + t.Minute()
	for _, m := range ms {
		from, to, err := parseRange(m.Range)
		if err != nil {
			continue
		}
		if from <= to {
			if minute >= from && minute < to {
				return m, true
			}
		} else if minute >= from || minute < to { // crosses midnight
			return m, true
		}
	}
	return TimeMultiplier{}, false
}

// parseRange parses "HH:MM-HH:MM" into minutes of day.
func parseRange(s string) (from, to int, err error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("格式须为 HH:MM-HH:MM，得到 %q", s)
	}
	p := func(v string) (int, error) {
		var h, m int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d:%d", &h, &m); err != nil || h < 0 || h > 24 || m < 0 || m > 59 {
			return 0, fmt.Errorf("时间 %q 不合法", v)
		}
		return h*60 + m, nil
	}
	if from, err = p(parts[0]); err != nil {
		return
	}
	if to, err = p(parts[1]); err != nil {
		return
	}
	if from == to {
		return 0, 0, errors.New("起止时间不能相同")
	}
	return from, to, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round1(v float64) float64 { return math.Round(v*10) / 10 }
