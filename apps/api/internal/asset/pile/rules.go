package pile

import (
	"regexp"
	"strings"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

var codeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{2,64}$`)

// Pile status values (contract PileStatusEnum).
const (
	Available = "available"
	Charging  = "charging"
	Offline   = "offline"
	Faulted   = "faulted"
	Disabled  = "disabled"
)

// NormalizeCode validates the OCPP ChargePoint identity (contract pattern ^[A-Za-z0-9_-]{2,64}$).
func NormalizeCode(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !codeRe.MatchString(s) {
		return "", httpx.BadRequest("pile_code 须为 2–64 位字母、数字、下划线或连字符")
	}
	return s, nil
}

// CheckManualStatus only lets an operator disable a pile or restore it to
// offline; available / charging / faulted are reported by the pile itself.
func CheckManualStatus(target string) error {
	switch target {
	case Disabled, Offline:
		return nil
	}
	return httpx.BadRequest("status 手动只能设为 disabled 或 offline")
}
