package charging

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

// Register mounts /charging/* (live piles, transactions, review, remote start/stop, summary, export).
// Implemented in phase 3.
func Register(g *gin.RouterGroup, a *app.App, billing BillingHook) {}
