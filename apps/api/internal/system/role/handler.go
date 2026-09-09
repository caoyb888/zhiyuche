// Package role manages roles, their permission sets, and exposes the permission registry.
package role

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

// Register mounts /system/roles and /system/permissions. Implemented in phase 1 second half.
func Register(g *gin.RouterGroup, a *app.App) {}
