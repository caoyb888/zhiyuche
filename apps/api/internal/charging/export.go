package charging

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
)

const (
	xlsxMIME    = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	exportSheet = "充电记录"
	timeLayout  = "2006-01-02 15:04:05"
)

// ExportHeaders is the column order of the xlsx export.
var ExportHeaders = []string{
	"事务号", "充电桩", "桩编号", "枪号", "卡号", "用户", "部门", "车牌", "绑定方式", "开始", "结束", "时长(分)",
	"电量(kWh)", "单价(元/kWh)", "费用(元)", "扣费账户", "BMS起始SOC", "BMS结束SOC", "BMS估算(kWh)", "偏差(%)",
	"状态", "复核", "复核备注", "停止原因",
}

// buildExport writes the transactions into a workbook (header + one row each).
func buildExport(txs []Transaction) (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName(f.GetSheetName(0), exportSheet); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(exportSheet, "A1", ptrAny(ExportHeaders)); err != nil {
		return nil, err
	}
	for i, t := range txs {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(exportSheet, cell, ptrAny(ExportRow(t))); err != nil {
			return nil, err
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	last, _ := excelize.CoordinatesToCellName(len(ExportHeaders), 1)
	if err := f.SetCellStyle(exportSheet, "A1", last, style); err != nil {
		return nil, err
	}
	lastCol, _ := excelize.ColumnNumberToName(len(ExportHeaders))
	_ = f.SetColWidth(exportSheet, "A", lastCol, 16)
	return f, nil
}

// ExportRow renders one transaction as the export columns.
func ExportRow(t Transaction) []string {
	user, dept, plate := "", deref(t.DeptName), ""
	if t.User != nil {
		user = t.User.Name
	}
	if t.Vehicle != nil {
		plate = t.Vehicle.PlateNo
	}
	return []string{
		t.TxNo, t.PileName, t.PileCode, fmt.Sprint(t.ConnectorID), t.IDTag, user, dept, plate, bindText(t.BindMethod),
		t.StartAt.In(approval.Shanghai).Format(timeLayout), fmtOptTime(t.EndAt), fmtOpt(t.DurationMin, 0),
		fmtOpt(t.Kwh, 3), fmtOpt(t.UnitPrice, 4), fmtOpt(t.Cost, 2), attributionText(t.Attribution),
		fmtOpt(t.BmsSocStart, 1), fmtOpt(t.BmsSocEnd, 1), fmtOpt(t.BmsKwhEst, 3), fmtOpt(t.DeviationPct, 2),
		statusText(t.Status), reviewText(t.ReviewStatus), deref(t.ReviewNote), deref(t.StopReason),
	}
}

func bindText(p *string) string {
	switch deref(p) {
	case BindManual:
		return "手动指定"
	case BindLocation:
		return "位置匹配"
	case BindRecentTrip:
		return "最近行程"
	case BindCard:
		return "仅刷卡人"
	case BindNone:
		return "未绑定"
	}
	return deref(p)
}

func attributionText(p *string) string {
	switch deref(p) {
	case "employee":
		return "员工"
	case "department":
		return "部门"
	case "enterprise":
		return "企业"
	}
	return deref(p)
}

func statusText(s string) string {
	switch s {
	case StatusCharging:
		return "充电中"
	case StatusEnded:
		return "已结束"
	case StatusSettled:
		return "已结算"
	case StatusCancelled:
		return "已取消"
	}
	return s
}

func reviewText(s string) string {
	switch s {
	case ReviewNone:
		return "无需复核"
	case ReviewPending:
		return "待复核"
	case ReviewApproved:
		return "已通过"
	case ReviewRejected:
		return "已拒绝"
	}
	return s
}

func fmtOpt(v *float64, prec int) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.*f", prec, *v)
}

func fmtOptTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(approval.Shanghai).Format(timeLayout)
}

func ptrAny(ss []string) *[]any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return &out
}
