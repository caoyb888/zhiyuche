// Package auditlog exposes read-only queries over audit_logs.
package auditlog

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

// Register mounts /system/audit-logs. Implemented in phase 1 second half.
func Register(g *gin.RouterGroup, a *app.App) {}
