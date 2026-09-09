package trip

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
)

const (
	xlsxMIME      = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	exportSheet   = "行程"
	exportMaxRows = 5000
	timeLayout    = "2006-01-02 15:04:05"
)

// ExportHeaders is the column order of the xlsx export.
var ExportHeaders = []string{
	"行程号", "车牌", "驾驶员", "部门", "类型", "事由", "开始", "结束", "时长(分)", "里程(km)", "耗电(kWh)",
	"百公里电耗(kWh)", "均速(km/h)", "最高(km/h)", "急加速", "急刹", "灯牌", "偏离", "状态",
}

// buildExport writes the trips into a workbook (header + one row per trip).
func buildExport(trips []Trip) (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName(f.GetSheetName(0), exportSheet); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(exportSheet, "A1", ptrAny(ExportHeaders)); err != nil {
		return nil, err
	}
	for i, t := range trips {
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

// ExportRow renders one trip as the export columns.
func ExportRow(t Trip) []string {
	driver, dept := "", ""
	if t.Driver != nil {
		driver = t.Driver.Name
		if t.Driver.DeptName != nil {
			dept = *t.Driver.DeptName
		}
	}
	purpose := ""
	if t.Purpose != nil {
		purpose = *t.Purpose
	} else if t.Approval != nil {
		purpose = t.Approval.PurposeDetail
	}
	return []string{
		t.TripNo, t.Vehicle.PlateNo, driver, dept, tripTypeText(t.TripType), purpose,
		t.StartAt.In(approval.Shanghai).Format(timeLayout), fmtOptTime(t.EndAt),
		fmtOpt(t.DurationMin, 0), fmtOpt(t.DistanceKm, 1), fmtOpt(t.EnergyKwh, 2), fmtOpt(t.EnergyPer100Km, 2),
		fmtOpt(t.AvgSpeed, 1), fmtOpt(t.MaxSpeed, 1), fmt.Sprint(t.HarshAccel), fmt.Sprint(t.HarshBrake),
		roofText(t.RoofSignStatus), boolText(t.DeviationFlag), statusText(t.Status),
	}
}

func tripTypeText(s string) string {
	switch s {
	case approval.TripTypeOfficial:
		return "公务用车"
	case approval.TripTypeDaily:
		return "日常用车"
	}
	return s
}

func roofText(s string) string {
	switch s {
	case "on":
		return "亮"
	case "off":
		return "灭"
	}
	return "未知"
}

func statusText(s string) string {
	switch s {
	case StatusOngoing:
		return "进行中"
	case StatusCompleted:
		return "已完成"
	case StatusCancelled:
		return "已作废"
	}
	return s
}

func boolText(b bool) string {
	if b {
		return "是"
	}
	return "否"
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
