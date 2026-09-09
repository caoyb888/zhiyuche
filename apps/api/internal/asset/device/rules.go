package device

import (
	"regexp"
	"strings"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

var serialRe = regexp.MustCompile(`^[A-Za-z0-9_-]{4,64}$`)

// NormalizeSerial trims and validates a gateway serial (contract pattern ^[A-Za-z0-9_-]{4,64}$).
func NormalizeSerial(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !serialRe.MatchString(s) {
		return "", httpx.BadRequest("serial_no 须为 4–64 位字母、数字、下划线或连字符")
	}
	return s, nil
}

// IsOnline reports whether a device reported within vstatus.OfflineAfter.
func IsOnline(last *time.Time, now time.Time) bool {
	return last != nil && now.Sub(*last) < vstatus.OfflineAfter
}

// CheckUnbind refuses to detach a gateway from a vehicle that is on a trip.
func CheckUnbind(vehicleStatus string, hasTrip bool) error {
	if vehicleStatus == vstatus.InUse || hasTrip {
		return httpx.Conflict("车辆在途，不能解绑设备")
	}
	return nil
}

// CheckBind decides whether device (currently bound to cur, nil if free) may be
// bound to target, given the id of the device already attached to target (nil if none).
// A device bound elsewhere must be unbound first; binding to its own vehicle is a no-op.
func CheckBind(cur, target *string, targetDevice, self *string) (noop bool, err error) {
	if cur != nil {
		if *cur == *target {
			return true, nil
		}
		return false, httpx.Conflict("设备已绑定其他车辆，请先解绑")
	}
	if targetDevice != nil && (self == nil || *targetDevice != *self) {
		return false, httpx.Conflict("目标车辆已绑定其他设备")
	}
	return false, nil
}
