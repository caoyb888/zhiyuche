// Package approval implements the official-vehicle request workflow:
// pre-check, one/two-level approval steps, rules, cancellation. Implemented in phase 2.
package approval

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

func Register(g *gin.RouterGroup, a *app.App) {}
