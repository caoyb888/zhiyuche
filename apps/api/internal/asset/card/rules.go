package card

import (
	"regexp"
	"strings"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

var uidRe = regexp.MustCompile(`^[A-Fa-f0-9]{8,32}$`)

// Card status values.
const (
	Active   = "active"
	Lost     = "lost"
	Disabled = "disabled"
)

// NormalizeUID validates a card uid (contract pattern ^[A-Fa-f0-9]{8,32}$) and
// returns it upper-cased, which is how it is stored and matched.
func NormalizeUID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !uidRe.MatchString(s) {
		return "", httpx.BadRequest("card_uid 须为 8–32 位十六进制字符")
	}
	return strings.ToUpper(s), nil
}
