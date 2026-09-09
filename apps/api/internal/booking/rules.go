package booking

import (
	"errors"
	"fmt"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
)

// Pure decision logic: the reserved window, the status machine and the labels
// used in audit summaries and conflict messages. Unit-tested without a database.

const (
	// MinDuration / MaxDuration bound the reserved window.
	MinDuration = 5 * time.Minute
	MaxDuration = 7 * 24 * time.Hour
	// PastLimit is how far back a booking may be back-filled (电话记录补录).
	PastLimit = 30 * 24 * time.Hour
	// ConflictBuffer is the turnaround kept between two holds of one vehicle;
	// same value as the approval module so both agree on what "free" means.
	ConflictBuffer = 15 * time.Minute
)

// ErrTransition is returned by NextStatus for an event the status forbids.
var ErrTransition = errors.New("当前状态不允许该操作")

// ValidateWindow checks the reserved window against now (Shanghai wall clock
// is irrelevant here — only durations are compared).
func ValidateWindow(start, end, now time.Time) error {
	d := end.Sub(start)
	if d <= 0 {
		return errors.New("预约结束时间须晚于开始时间")
	}
	if d < MinDuration {
		return fmt.Errorf("预约时长至少 %d 分钟", int(MinDuration.Minutes()))
	}
	if d > MaxDuration {
		return fmt.Errorf("预约时长不能超过 %d 天", int(MaxDuration.Hours()/24))
	}
	if start.Before(now.Add(-PastLimit)) {
		return fmt.Errorf("预约开始时间不能早于 %d 天前", int(PastLimit.Hours()/24))
	}
	return nil
}

// NextStatus is the status machine: reserved →(depart) departed →(complete)
// completed; reserved →(cancel) cancelled. Nothing leaves a final status.
func NextStatus(status, event string) (string, error) {
	switch event {
	case EvDepart:
		if status == StatusReserved {
			return StatusDeparted, nil
		}
	case EvComplete:
		if status == StatusDeparted {
			return StatusCompleted, nil
		}
	case EvCancel:
		if status == StatusReserved {
			return StatusCancelled, nil
		}
	}
	return "", fmt.Errorf("%w: %s on %s", ErrTransition, event, status)
}

// Editable reports whether the booking's fields may still be changed.
func Editable(status string) bool { return status == StatusReserved }

var sourceLabels = map[string]string{
	SourcePhone:  "电话预约",
	SourceDirect: "直接预约",
}

var statusLabels = map[string]string{
	StatusReserved:  "待出车",
	StatusDeparted:  "已出车",
	StatusCompleted: "已完成",
	StatusCancelled: "已取消",
}

// SourceLabel / StatusLabel fall back to the raw code for unknown values.
func SourceLabel(s string) string {
	if v, ok := sourceLabels[s]; ok {
		return v
	}
	return s
}

func StatusLabel(s string) string {
	if v, ok := statusLabels[s]; ok {
		return v
	}
	return s
}

// FormatWindow renders a window in the business time zone for user-facing
// conflict messages: `09-10 08:00 → 12:00`.
func FormatWindow(start, end time.Time) string {
	s := start.In(approval.Shanghai)
	e := end.In(approval.Shanghai)
	right := e.Format("01-02 15:04")
	if s.Year() == e.Year() && s.YearDay() == e.YearDay() {
		right = e.Format("15:04")
	}
	return s.Format("01-02 15:04") + " → " + right
}

// ConflictMessage renders the 409 body message for the first few conflicts.
func ConflictMessage(plate string, cs []Conflict) string {
	msg := "车辆 " + plate + " 在该时段已被占用："
	for i, c := range cs {
		if i == 3 {
			msg += fmt.Sprintf("等 %d 条", len(cs))
			break
		}
		if i > 0 {
			msg += "；"
		}
		kind := "预约"
		if c.Kind == "approval" {
			kind = "申请"
		}
		msg += fmt.Sprintf("%s %s（%s，%s）", kind, c.No, c.Who, FormatWindow(c.Start, c.End))
	}
	return msg
}
