// Package tenant manages tenants (platform super admin only).
package tenant

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

// Register mounts /system/tenants. Implemented in phase 1 second half.
func Register(g *gin.RouterGroup, a *app.App) {}
