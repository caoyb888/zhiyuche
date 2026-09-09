package user

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// registerImportExport mounts Excel import/export. The real implementation
// (excelize) lands in the second half of phase 1; the routes exist so the
// contract and permission codes are fixed now.
func registerImportExport(r *gin.RouterGroup, h *handler) {
	r.POST("/import", auth.Require("system:user:import"), h.importUsers)
	r.GET("/export", auth.Require("system:user:export"), h.exportUsers)
	r.GET("/import-template", auth.Require("system:user:import"), h.importTemplate)
}

func (h *handler) importUsers(c *gin.Context) {
	httpx.Fail(c, &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"})
}

func (h *handler) exportUsers(c *gin.Context) {
	httpx.Fail(c, &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"})
}

func (h *handler) importTemplate(c *gin.Context) {
	httpx.Fail(c, &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"})
}
