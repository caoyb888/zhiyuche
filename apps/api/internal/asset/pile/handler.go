// Package pile manages the charge pile archive (/assets/charge-piles). Live OCPP state arrives in phase 3.
package pile

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
)

func Register(g *gin.RouterGroup, a *app.App) {}
