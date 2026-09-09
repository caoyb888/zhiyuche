package user

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// registerImportExport mounts the Excel endpoints (logic in excel.go).
func registerImportExport(r *gin.RouterGroup, h *handler) {
	r.POST("/import", auth.Require("system:user:import"), h.importUsers)
	r.GET("/export", auth.Require("system:user:export"), h.exportUsers)
	r.GET("/import-template", auth.Require("system:user:import"), h.importTemplate)
}

// importTemplate serves the xlsx template with a header and an example row.
func (h *handler) importTemplate(c *gin.Context) {
	f, err := buildImportTemplate()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	writeXLSX(c, f, "user-import-template.xlsx")
}

// importUsers parses the uploaded workbook (multipart field "file", ≤5MB,
// ≤2000 rows) and creates users row by row; partial success is normal.
func (h *handler) importUsers(c *gin.Context) {
	// cap the whole multipart body a little above the file limit
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, importMaxBytes+512*1024)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Fail(c, httpx.BadRequest("文件不能超过 5MB"))
			return
		}
		httpx.Fail(c, httpx.BadRequest("缺少文件字段 file"))
		return
	}
	if fh.Size > importMaxBytes {
		httpx.Fail(c, httpx.BadRequest("文件不能超过 5MB"))
		return
	}
	src, err := fh.Open()
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("无法读取上传文件"))
		return
	}
	defer src.Close()

	f, err := excelize.OpenReader(src)
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("无法解析 Excel 文件，请上传 xlsx 格式"))
		return
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		httpx.Fail(c, httpx.BadRequest("Excel 文件没有工作表"))
		return
	}
	raw, err := f.GetRows(sheets[0])
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("无法读取工作表："+err.Error()))
		return
	}
	rows, err := parseImportRows(raw)
	if err != nil {
		httpx.Fail(c, httpx.BadRequest(err.Error()))
		return
	}

	res, err := h.svc.Import(c.Request.Context(), auth.TenantID(c), rows, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{
		Module: module, Action: "import", TargetType: "user",
		Summary: fmt.Sprintf("导入用户：共 %d 行，成功 %d，失败 %d", res.Total, res.Success, res.Failed),
		After:   res,
	})
	httpx.OK(c, res)
}

// exportUsers streams an xlsx of the users matching the list filters.
func (h *handler) exportUsers(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	users, err := h.svc.Export(c.Request.Context(), auth.TenantID(c), q)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	f, err := buildExport(users)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	writeXLSX(c, f, "users-"+time.Now().Format("20060102-150405")+".xlsx")
}

func writeXLSX(c *gin.Context, f *excelize.File, filename string) {
	defer f.Close()
	buf, err := f.WriteToBuffer()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, xlsxMIME, buf.Bytes())
}
