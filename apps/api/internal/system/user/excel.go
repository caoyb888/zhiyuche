package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

// Excel import / export: the pure parsing, validation and workbook-building
// logic lives here (unit-testable); the HTTP glue is in importexport.go.

const (
	xlsxMIME       = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	importSheet    = "用户"
	importMaxBytes = 5 << 20 // 5MB
	importMaxRows  = 2000
	exportMaxRows  = 5000
	timeLayout     = "2006-01-02 15:04:05"
)

// importHeaders is the template header row; "*" marks required columns.
var importHeaders = []string{"用户名*", "姓名*", "手机号", "邮箱", "工号", "部门名称", "角色代码"}

// importExample is the sample row shipped in the template.
var importExample = []string{"zhangsan", "张三", "13800000000", "zhangsan@example.com", "E0001", "技术部", "employee,approver"}

var exportHeaders = []string{"用户名", "姓名", "手机号", "邮箱", "工号", "部门", "角色", "状态", "最后登录", "创建时间"}

// ImportResult is the response of POST /system/users/import.
type ImportResult struct {
	Total   int           `json:"total"`
	Success int           `json:"success"`
	Failed  int           `json:"failed"`
	Errors  []ImportError `json:"errors"`
}

type ImportError struct {
	Row      int    `json:"row"` // Excel 行号（从 2 起）
	Username string `json:"username,omitempty"`
	Message  string `json:"message"`
}

// importRow is one parsed data row of the import sheet.
type importRow struct {
	Row        int
	Username   string
	Name       string
	Phone      string
	Email      string
	EmployeeNo string
	DeptName   string
	RoleCodes  []string
}

// buildImportTemplate creates the xlsx template: header + one example row.
func buildImportTemplate() (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName(f.GetSheetName(0), importSheet); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(importSheet, "A1", ptrAny(importHeaders)); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(importSheet, "A2", ptrAny(importExample)); err != nil {
		return nil, err
	}
	if err := styleHeader(f, importSheet, len(importHeaders)); err != nil {
		return nil, err
	}
	return f, nil
}

// parseImportRows turns the raw sheet (as returned by excelize GetRows) into
// data rows. The first row must be the template header; blank rows are
// skipped; Row is the 1-based Excel row number (data starts at 2).
func parseImportRows(rows [][]string) ([]importRow, error) {
	if len(rows) == 0 {
		return nil, errors.New("文件为空")
	}
	header := rows[0]
	if !strings.HasPrefix(strings.TrimSpace(cell(header, 0)), "用户名") || !strings.HasPrefix(strings.TrimSpace(cell(header, 1)), "姓名") {
		return nil, errors.New("表头不符，请使用下载的导入模板")
	}
	out := []importRow{}
	for i, r := range rows[1:] {
		if isBlankRow(r) {
			continue
		}
		out = append(out, importRow{
			Row:        i + 2,
			Username:   strings.TrimSpace(cell(r, 0)),
			Name:       strings.TrimSpace(cell(r, 1)),
			Phone:      strings.TrimSpace(cell(r, 2)),
			Email:      strings.TrimSpace(cell(r, 3)),
			EmployeeNo: strings.TrimSpace(cell(r, 4)),
			DeptName:   strings.TrimSpace(cell(r, 5)),
			RoleCodes:  splitCodes(cell(r, 6)),
		})
		if len(out) > importMaxRows {
			return nil, fmt.Errorf("单次最多导入 %d 行", importMaxRows)
		}
	}
	return out, nil
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// validateImportRow checks the per-row constraints that do not need the
// database; it returns a user-facing message, or "" when the row is fine.
func validateImportRow(r importRow) string {
	switch n := utf8.RuneCountInString(r.Username); {
	case n == 0:
		return "用户名不能为空"
	case n < 2 || n > 64:
		return "用户名长度须为 2-64 个字符"
	}
	switch n := utf8.RuneCountInString(r.Name); {
	case n == 0:
		return "姓名不能为空"
	case n > 64:
		return "姓名不能超过 64 个字符"
	}
	if utf8.RuneCountInString(r.Phone) > 32 {
		return "手机号不能超过 32 个字符"
	}
	if r.Email != "" && (utf8.RuneCountInString(r.Email) > 128 || !emailPattern.MatchString(r.Email)) {
		return "邮箱格式不正确"
	}
	if utf8.RuneCountInString(r.EmployeeNo) > 64 {
		return "工号不能超过 64 个字符"
	}
	return ""
}

// Import creates one user per row through Service.Create (default password);
// failures are collected per row and never roll back earlier rows.
func (s *Service) Import(ctx context.Context, tenantID uuid.UUID, rows []importRow, actor *auth.Principal) (*ImportResult, error) {
	depts, err := s.store.deptIDsByName(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	roles, err := s.store.roleIDsByCode(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	res := &ImportResult{Total: len(rows), Errors: []ImportError{}}
	seen := map[string]int{}
	for _, r := range rows {
		if msg := prepareImportRow(r, seen, depts, roles); msg != "" {
			res.fail(r, msg)
			continue
		}
		req := importRequest(r, depts, roles)
		if _, err := s.Create(ctx, tenantID, req, actor); err != nil {
			res.fail(r, errorMessage(err))
			continue
		}
		res.Success++
	}
	return res, nil
}

// prepareImportRow runs the row validations that need the lookup tables and
// the in-file duplicate check. Only a row that passes every check claims its
// username in seen, so a later row reusing the username of a rejected row is
// judged on its own.
func prepareImportRow(r importRow, seen map[string]int, depts map[string][]uuid.UUID, roles map[string]uuid.UUID) string {
	if msg := validateImportRow(r); msg != "" {
		return msg
	}
	if prev, dup := seen[r.Username]; dup {
		return fmt.Sprintf("用户名与第 %d 行重复", prev)
	}
	if r.DeptName != "" {
		switch ids := depts[r.DeptName]; len(ids) {
		case 0:
			return "部门不存在：" + r.DeptName
		case 1:
		default:
			return "部门名称不唯一：" + r.DeptName
		}
	}
	for _, code := range r.RoleCodes {
		if _, ok := roles[code]; !ok {
			return "角色代码不存在：" + code
		}
	}
	seen[r.Username] = r.Row
	return ""
}

func importRequest(r importRow, depts map[string][]uuid.UUID, roles map[string]uuid.UUID) CreateRequest {
	req := CreateRequest{
		Username:   r.Username,
		Name:       r.Name,
		Phone:      optional(r.Phone),
		Email:      optional(r.Email),
		EmployeeNo: optional(r.EmployeeNo),
	}
	if r.DeptName != "" {
		id := depts[r.DeptName][0]
		req.DeptID = &id
	}
	for _, code := range r.RoleCodes {
		req.RoleIDs = append(req.RoleIDs, roles[code])
	}
	return req
}

func (res *ImportResult) fail(r importRow, msg string) {
	res.Failed++
	res.Errors = append(res.Errors, ImportError{Row: r.Row, Username: r.Username, Message: msg})
}

// Export returns the users matching the list filters, up to exportMaxRows, oldest first.
func (s *Service) Export(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]User, error) {
	rows, _, err := s.store.list(ctx, tenantID, q, pagination.Query{Page: 1, PageSize: exportMaxRows, SortBy: "created_at"})
	return rows, err
}

// buildExport writes the users into a workbook.
func buildExport(users []User) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := importSheet
	if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(sheet, "A1", ptrAny(exportHeaders)); err != nil {
		return nil, err
	}
	for i, u := range users {
		cellName, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(sheet, cellName, ptrAny(exportRow(u))); err != nil {
			return nil, err
		}
	}
	if err := styleHeader(f, sheet, len(exportHeaders)); err != nil {
		return nil, err
	}
	return f, nil
}

func exportRow(u User) []string {
	roles := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		roles = append(roles, r.Name)
	}
	return []string{
		u.Username, u.Name, deref(u.Phone), deref(u.Email), deref(u.EmployeeNo), deref(u.DeptName),
		strings.Join(roles, "、"), statusText(u.Status), formatTime(u.LastLoginAt), u.CreatedAt.Local().Format(timeLayout),
	}
}

func statusText(s string) string {
	switch s {
	case "active":
		return "正常"
	case "disabled":
		return "停用"
	case "locked":
		return "锁定"
	}
	return s
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Local().Format(timeLayout)
}

// ---- store lookups used only by import

func (s *store) deptIDsByName(ctx context.Context, tenantID uuid.UUID) (map[string][]uuid.UUID, error) {
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Name string    `db:"name"`
	}
	if err := pgxscan.Select(ctx, s.db, &rows, `SELECT id, name FROM departments WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID); err != nil {
		return nil, err
	}
	out := map[string][]uuid.UUID{}
	for _, r := range rows {
		out[strings.TrimSpace(r.Name)] = append(out[strings.TrimSpace(r.Name)], r.ID)
	}
	return out, nil
}

func (s *store) roleIDsByCode(ctx context.Context, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Code string    `db:"code"`
	}
	if err := pgxscan.Select(ctx, s.db, &rows, `SELECT id, code FROM roles WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID); err != nil {
		return nil, err
	}
	out := make(map[string]uuid.UUID, len(rows))
	for _, r := range rows {
		out[r.Code] = r.ID
	}
	return out, nil
}

// ---- small helpers

func styleHeader(f *excelize.File, sheet string, cols int) error {
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	last, err := excelize.CoordinatesToCellName(cols, 1)
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", last, style); err != nil {
		return err
	}
	lastCol, err := excelize.ColumnNumberToName(cols)
	if err != nil {
		return err
	}
	return f.SetColWidth(sheet, "A", lastCol, 22)
}

func ptrAny(ss []string) *[]any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return &out
}

func cell(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

func isBlankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// splitCodes splits "a,b" (ASCII or Chinese comma / semicolon), trims and de-duplicates.
func splitCodes(s string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == '；' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, dup := seen[part]; dup {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func errorMessage(err error) string {
	var ae *httpx.AppError
	if errors.As(err, &ae) {
		return ae.Message
	}
	return "内部错误"
}
