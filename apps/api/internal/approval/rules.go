package approval

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Pure decision logic of the workflow: which requests need a second level,
// the night-window overlap test, the status state machine and the daily
// sequence numbers. Everything here is unit-tested without a database.

// Shanghai is the business time zone (apply numbers, night windows, daily summaries).
var Shanghai = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}()

var hhmm = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// ParseHHMM validates "HH:MM" and returns minutes since midnight.
func ParseHHMM(s string) (int, error) {
	if !hhmm.MatchString(s) {
		return 0, fmt.Errorf("时间格式须为 HH:MM：%q", s)
	}
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	return h*60 + m, nil
}

// NightOverlap reports whether [start, end) touches the nightly window
// [nightStart, nightEnd) of any day it spans. The window may cross midnight
// (22:00 → 06:00). Invalid HH:MM strings never match.
func NightOverlap(start, end time.Time, nightStart, nightEnd string) bool {
	if !end.After(start) {
		return false
	}
	ns, err1 := ParseHHMM(nightStart)
	ne, err2 := ParseHHMM(nightEnd)
	if err1 != nil || err2 != nil || ns == ne {
		return false
	}
	s := start.In(Shanghai)
	e := end.In(Shanghai)
	// windows anchored on each calendar day from the day before start to the day of end
	day := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, Shanghai).AddDate(0, 0, -1)
	last := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, Shanghai)
	for !day.After(last) {
		ws := day.Add(time.Duration(ns) * time.Minute)
		we := day.Add(time.Duration(ne) * time.Minute)
		if !we.After(ws) {
			we = we.AddDate(0, 0, 1)
		}
		if ws.Before(e) && we.After(s) {
			return true
		}
		day = day.AddDate(0, 0, 1)
	}
	return false
}

// Level2Input is what the rules look at for one request.
type Level2Input struct {
	PlannedKm       *float64
	Start, End      time.Time
	TripType        string
	VehicleHomeDept *uuid.UUID // nil when no vehicle or vehicle has no home dept
	ApplicantDept   *uuid.UUID
}

// Level2Reasons lists why the request needs a second approval level (empty =
// one level is enough). Rules are ignored when disabled.
func (r Rules) Level2Reasons(in Level2Input) []string {
	out := []string{}
	if !r.Enabled {
		return out
	}
	if r.Level2Km != nil && in.PlannedKm != nil && *in.PlannedKm >= *r.Level2Km {
		out = append(out, fmt.Sprintf("预计里程 %.1f km ≥ %.1f km", *in.PlannedKm, *r.Level2Km))
	}
	if r.Level2Night && NightOverlap(in.Start, in.End, r.NightStart, r.NightEnd) {
		out = append(out, fmt.Sprintf("计划时段与夜间 %s-%s 重叠", r.NightStart, r.NightEnd))
	}
	if r.Level2CrossDept && in.VehicleHomeDept != nil && (in.ApplicantDept == nil || *in.ApplicantDept != *in.VehicleHomeDept) {
		out = append(out, "跨部门用车（车辆归属部门与申请人部门不同）")
	}
	for _, t := range r.Level2TripTypes {
		if t == in.TripType {
			out = append(out, "用车类型 "+tripTypeLabel(t)+" 需二级审批")
			break
		}
	}
	return out
}

func tripTypeLabel(t string) string {
	switch t {
	case TripTypeOfficial:
		return "公务用车"
	case TripTypeDaily:
		return "日常用车"
	}
	return t
}

// Workflow events driving the state machine.
const (
	EvApprove    = "approve"
	EvReject     = "reject"
	EvCancel     = "cancel"
	EvTripStart  = "trip_start"
	EvTripEnd    = "trip_end"
	EvTripCancel = "trip_cancel"
	EvExpire     = "expire"
)

// ErrTransition is returned for a status/event pair the state machine forbids.
var ErrTransition = errors.New("invalid transition")

// NextStatus returns the status after applying event to a request in status
// with the given level_required / current_step.
//
//	pending_l1 --approve--> pending_l2 (level 2) | approved
//	pending_l2 --approve--> approved
//	pending_*  --reject---> rejected
//	pending_*, approved --cancel--> cancelled
//	approved --trip_start--> in_use --trip_end--> completed
//	in_use --trip_cancel--> approved
//	pending_*, approved --expire--> expired
func NextStatus(status, event string, levelRequired, currentStep int) (string, error) {
	pending := status == StatusPendingL1 || status == StatusPendingL2
	switch event {
	case EvApprove:
		if status == StatusPendingL1 && levelRequired >= 2 && currentStep < levelRequired {
			return StatusPendingL2, nil
		}
		if pending {
			return StatusApproved, nil
		}
	case EvReject:
		if pending {
			return StatusRejected, nil
		}
	case EvCancel:
		if pending || status == StatusApproved {
			return StatusCancelled, nil
		}
	case EvTripStart:
		if status == StatusApproved {
			return StatusInUse, nil
		}
	case EvTripEnd:
		if status == StatusInUse {
			return StatusCompleted, nil
		}
	case EvTripCancel:
		if status == StatusInUse {
			return StatusApproved, nil
		}
	case EvExpire:
		if pending || status == StatusApproved {
			return StatusExpired, nil
		}
	}
	return "", fmt.Errorf("%w: %s on %s", ErrTransition, event, status)
}

// FormatNo renders "<prefix>-YYYYMMDD-<seq>" with seq zero-padded to width.
func FormatNo(prefix string, day time.Time, seq, width int) string {
	return fmt.Sprintf("%s-%s-%0*d", prefix, day.In(Shanghai).Format("20060102"), width, seq)
}

// DayPrefix is the part of a number shared by every row of one day.
func DayPrefix(prefix string, day time.Time) string {
	return prefix + "-" + day.In(Shanghai).Format("20060102")
}

// NextNo allocates the next daily sequence number for column of table inside
// tx. A transaction-scoped advisory lock keyed by the day prefix serialises
// concurrent allocations, so two requests never receive the same number.
// The unique indexes on apply_no / trip_no are global, so the sequence is
// per day across tenants (still strictly increasing inside a tenant).
func NextNo(ctx context.Context, tx pgx.Tx, table, column, prefix string, day time.Time, width int) (string, error) {
	dp := DayPrefix(prefix, day)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, table+":"+dp); err != nil {
		return "", err
	}
	var seq int
	q := fmt.Sprintf(`SELECT COALESCE(MAX((regexp_match(%s, '-(\d+)$'))[1]::int), 0) + 1 FROM %s WHERE %s LIKE $1`, column, table, column)
	if err := tx.QueryRow(ctx, q, dp+"-%").Scan(&seq); err != nil {
		return "", err
	}
	return FormatNo(prefix, day, seq, width), nil
}

// ValidateRulesUpdate checks the HH:MM fields; returns a user-facing message or "".
func ValidateRulesUpdate(u *RulesUpdate) string {
	for _, f := range []struct {
		name string
		v    *string
	}{{"night_start", u.NightStart}, {"night_end", u.NightEnd}} {
		if f.v != nil {
			if _, err := ParseHHMM(strings.TrimSpace(*f.v)); err != nil {
				return f.name + " " + err.Error()
			}
		}
	}
	return ""
}
