// Package vehicle manages the vehicle archive (/assets/vehicles) and exposes
// live status/telemetry queries. Implemented in phase 2.
package vehicle

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

func Register(g *gin.RouterGroup, a *app.App) {}
