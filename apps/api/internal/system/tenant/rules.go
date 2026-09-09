package tenant

import (
	"regexp"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"

	defaultAdminName = "管理员"
)

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)

// ValidateCode enforces the tenant code pattern from the contract.
func ValidateCode(code string) error {
	if !codeRe.MatchString(code) {
		return httpx.BadRequest("租户代码须为 2-32 位小写字母、数字或短横线，且以字母开头")
	}
	return nil
}

// Normalize fills the defaults of a create request (admin display name).
func Normalize(req CreateRequest) CreateRequest {
	if req.AdminName == "" {
		req.AdminName = defaultAdminName
	}
	return req
}

// Transition describes what a status change means for the tenant's sessions.
type Transition int

const (
	NoChange   Transition = iota
	Disable               // 吊销该租户全部会话
	Reactivate            // 只需让缓存快照失效
)

// StatusTransition decides, from the current status and the requested one,
// whether the change disables, re-activates or does nothing. The platform
// tenant can never be disabled.
func StatusTransition(current string, requested *string, isPlatform bool) (Transition, error) {
	if requested == nil || *requested == current {
		return NoChange, nil
	}
	if *requested == StatusDisabled {
		if isPlatform {
			return NoChange, httpx.BadRequest("平台租户不可停用")
		}
		return Disable, nil
	}
	return Reactivate, nil
}
