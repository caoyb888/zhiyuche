package vehicle

import (
	"fmt"
	"strconv"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// MaxTelemetryWindow bounds GET /assets/vehicles/{id}/telemetry.
const MaxTelemetryWindow = 7 * 24 * time.Hour

const (
	DefaultTelemetryLimit = 1000
	MaxTelemetryLimit     = 5000
)

// CheckManualStatus decides whether an operator may switch a vehicle to target.
// Only idle / maintenance / disabled can be set by hand, and never while the
// vehicle is on a trip (status in_use or an open current_trip_id).
func CheckManualStatus(current string, hasTrip bool, target string) error {
	switch target {
	case vstatus.Idle, vstatus.Maintenance, vstatus.Disabled:
	default:
		return httpx.BadRequest("status 只能为 idle / maintenance / disabled")
	}
	if current == vstatus.InUse || hasTrip {
		return httpx.Conflict("车辆在途，不能修改状态")
	}
	return nil
}

// CheckDeletable rejects deleting a vehicle that is on a trip or still
// referenced by an unfinished approval.
func CheckDeletable(current string, hasTrip bool, activeApprovals int64) error {
	if current == vstatus.InUse || hasTrip {
		return httpx.Conflict("车辆在途，不能删除")
	}
	if activeApprovals > 0 {
		return httpx.Conflict(fmt.Sprintf("存在 %d 条未完成的用车申请引用该车辆，不能删除", activeApprovals))
	}
	return nil
}

// CheckTelemetryRange validates the from/to window of the telemetry query.
func CheckTelemetryRange(from, to time.Time) error {
	if from.IsZero() || to.IsZero() {
		return httpx.BadRequest("from and to are required (RFC3339)")
	}
	if !to.After(from) {
		return httpx.BadRequest("to must be after from")
	}
	if to.Sub(from) > MaxTelemetryWindow {
		return httpx.BadRequest("time range must be within 7 days")
	}
	return nil
}

// ParseTelemetryLimit parses the limit query (default 1000, clamped to 5000).
func ParseTelemetryLimit(raw string) (int, error) {
	if raw == "" {
		return DefaultTelemetryLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, httpx.BadRequest("limit must be a positive integer")
	}
	if n > MaxTelemetryLimit {
		n = MaxTelemetryLimit
	}
	return n, nil
}
