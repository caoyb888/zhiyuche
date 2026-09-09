package billing

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
)

const (
	xlsxMIME     = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	summarySheet = "汇总"
	detailSheet  = "明细"
	timeLayout   = "2006-01-02 15:04:05"
)

// SummaryHeaders / DetailHeaders are the export column orders.
var (
	SummaryHeaders = []string{"部门", "行程数", "行程费", "充电数", "充电费", "罚金", "合计", "预算", "使用率"}
	DetailHeaders  = []string{"类型", "单号", "日期", "用户", "车牌", "数量", "金额"}
)

// buildExport writes the summary and detail sheets.
func buildExport(period string, rows []Settlement, lines []SettlementLine) (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName(f.GetSheetName(0), summarySheet); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet(detailSheet); err != nil {
		return nil, err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	writeSheet := func(sheet string, headers []string, data [][]any) error {
		if err := f.SetSheetRow(sheet, "A1", anyRow(headers)); err != nil {
			return err
		}
		for i, r := range data {
			cell, _ := excelize.CoordinatesToCellName(1, i+2)
			if err := f.SetSheetRow(sheet, cell, &r); err != nil {
				return err
			}
		}
		last, _ := excelize.CoordinatesToCellName(len(headers), 1)
		if err := f.SetCellStyle(sheet, "A1", last, bold); err != nil {
			return err
		}
		lastCol, _ := excelize.ColumnNumberToName(len(headers))
		return f.SetColWidth(sheet, "A", lastCol, 16)
	}
	summary := make([][]any, 0, len(rows))
	for _, r := range rows {
		summary = append(summary, SummaryRow(r))
	}
	if err := writeSheet(summarySheet, SummaryHeaders, summary); err != nil {
		return nil, err
	}
	detail := make([][]any, 0, len(lines))
	for _, l := range lines {
		detail = append(detail, DetailRow(l))
	}
	if err := writeSheet(detailSheet, DetailHeaders, detail); err != nil {
		return nil, err
	}
	f.SetActiveSheet(0)
	_ = f.SetDocProps(&excelize.DocProperties{Title: "结算单 " + period})
	return f, nil
}

// SummaryRow renders one settlement as summary columns.
func SummaryRow(r Settlement) []any {
	return []any{deptLabel(r), r.TripCount, r.TripCost, r.ChargeCount, r.ChargeCost, r.Penalty, r.Total, r.Budget, usageRate(r.Total, r.Budget)}
}

// DetailRow renders one line as detail columns.
func DetailRow(l SettlementLine) []any {
	qty := ""
	if l.Quantity != nil {
		unit := "km"
		if l.Kind == "charge" {
			unit = "kWh"
		}
		qty = fmt.Sprintf("%.2f %s", *l.Quantity, unit)
	}
	return []any{kindText(l.Kind), deref(l.RefNo), l.OccurredAt.In(engine.Location()).Format(timeLayout), deref(l.UserName), deref(l.VehiclePlate), qty, l.Amount}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func anyRow(ss []string) *[]any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return &out
}

func exportFilename(period string) string {
	return fmt.Sprintf("settlement-%s-%s.xlsx", period, time.Now().In(engine.Location()).Format("20060102"))
}
