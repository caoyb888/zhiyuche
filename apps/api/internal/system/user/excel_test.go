package user

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

func TestParseImportRows(t *testing.T) {
	rows := [][]string{
		{"用户名*", "姓名*", "手机号", "邮箱", "工号", "部门名称", "角色代码"},
		{" alice ", "爱丽丝", "13800000001", "a@x.com", "E1", "技术部", "employee, approver ,employee"},
		{"", "", ""}, // blank → skipped
		{"bob", "鲍勃"}, // short row (excelize trims trailing empty cells)
		{"carol", "卡罗", "", "", "", "", "finance；approver，employee"},
	}
	got, err := parseImportRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(got), got)
	}
	if got[0].Row != 2 || got[0].Username != "alice" || got[0].DeptName != "技术部" || strings.Join(got[0].RoleCodes, ",") != "employee,approver" {
		t.Fatalf("row 2 parsed wrong: %+v", got[0])
	}
	if got[1].Row != 4 || got[1].Username != "bob" || got[1].RoleCodes != nil || got[1].Phone != "" {
		t.Fatalf("row 4 parsed wrong: %+v", got[1])
	}
	if got[2].Row != 5 || strings.Join(got[2].RoleCodes, ",") != "finance,approver,employee" {
		t.Fatalf("row 5 parsed wrong: %+v", got[2])
	}
}

func TestParseImportRowsErrors(t *testing.T) {
	if _, err := parseImportRows(nil); err == nil {
		t.Fatal("empty sheet must fail")
	}
	if _, err := parseImportRows([][]string{{"name", "user"}}); err == nil {
		t.Fatal("wrong header must fail")
	}
	// header only → zero rows, no error
	if got, err := parseImportRows([][]string{{"用户名*", "姓名*"}}); err != nil || len(got) != 0 {
		t.Fatalf("header only: %v %v", got, err)
	}
	many := [][]string{{"用户名*", "姓名*"}}
	for i := 0; i < importMaxRows+1; i++ {
		many = append(many, []string{"u", "n"})
	}
	if _, err := parseImportRows(many); err == nil || !strings.Contains(err.Error(), "2000") {
		t.Fatalf("too many rows must fail with the limit, got %v", err)
	}
	if _, err := parseImportRows(many[:importMaxRows+1]); err != nil {
		t.Fatalf("exactly %d rows must pass: %v", importMaxRows, err)
	}
}

func TestValidateImportRow(t *testing.T) {
	ok := importRow{Username: "alice", Name: "A", Email: "a@b.co", Phone: "1"}
	if msg := validateImportRow(ok); msg != "" {
		t.Fatalf("valid row rejected: %s", msg)
	}
	cases := []struct {
		row  importRow
		want string
	}{
		{importRow{Name: "A"}, "用户名不能为空"},
		{importRow{Username: "a", Name: "A"}, "用户名长度"},
		{importRow{Username: "alice"}, "姓名不能为空"},
		{importRow{Username: "alice", Name: "A", Email: "not-an-email"}, "邮箱格式"},
		{importRow{Username: "alice", Name: "A", Phone: strings.Repeat("1", 33)}, "手机号"},
		{importRow{Username: "alice", Name: "A", EmployeeNo: strings.Repeat("x", 65)}, "工号"},
	}
	for _, c := range cases {
		if msg := validateImportRow(c.row); !strings.Contains(msg, c.want) {
			t.Errorf("%+v: got %q, want containing %q", c.row, msg, c.want)
		}
	}
}

func TestPrepareImportRow(t *testing.T) {
	dept := uuid.New()
	role := uuid.New()
	depts := map[string][]uuid.UUID{"技术部": {dept}, "重名部": {uuid.New(), uuid.New()}}
	roles := map[string]uuid.UUID{"employee": role}
	seen := map[string]int{}

	r := importRow{Row: 2, Username: "alice", Name: "A", DeptName: "技术部", RoleCodes: []string{"employee"}}
	if msg := prepareImportRow(r, seen, depts, roles); msg != "" {
		t.Fatalf("good row rejected: %s", msg)
	}
	req := importRequest(r, depts, roles)
	if req.DeptID == nil || *req.DeptID != dept || len(req.RoleIDs) != 1 || req.RoleIDs[0] != role || req.Phone != nil || req.Password != "" {
		t.Fatalf("request built wrong: %+v", req)
	}
	dup := importRow{Row: 3, Username: "alice", Name: "B"}
	if msg := prepareImportRow(dup, seen, depts, roles); !strings.Contains(msg, "第 2 行重复") {
		t.Fatalf("duplicate username: %q", msg)
	}
	if msg := prepareImportRow(importRow{Row: 4, Username: "bob", Name: "B", DeptName: "没有的部门"}, seen, depts, roles); !strings.Contains(msg, "部门不存在") {
		t.Fatalf("missing dept: %q", msg)
	}
	if msg := prepareImportRow(importRow{Row: 5, Username: "carol", Name: "C", DeptName: "重名部"}, seen, depts, roles); !strings.Contains(msg, "不唯一") {
		t.Fatalf("ambiguous dept: %q", msg)
	}
	if msg := prepareImportRow(importRow{Row: 6, Username: "dave", Name: "D", RoleCodes: []string{"nope"}}, seen, depts, roles); !strings.Contains(msg, "角色代码不存在") {
		t.Fatalf("missing role: %q", msg)
	}
	// a rejected row does not claim its username: the next "dave" is judged on its own
	if msg := prepareImportRow(importRow{Row: 7, Username: "dave", Name: "D"}, seen, depts, roles); msg != "" {
		t.Fatalf("username of a rejected row must be reusable, got %q", msg)
	}
	if msg := prepareImportRow(importRow{Row: 8, Username: "dave", Name: "D"}, seen, depts, roles); !strings.Contains(msg, "第 7 行重复") {
		t.Fatalf("duplicate of an accepted row: %q", msg)
	}
}

func TestImportTemplateRoundTrip(t *testing.T) {
	f, err := buildImportTemplate()
	if err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	g, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	sheets := g.GetSheetList()
	if len(sheets) != 1 || sheets[0] != importSheet {
		t.Fatalf("sheets = %v", sheets)
	}
	raw, err := g.GetRows(sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(raw[0], "|") != strings.Join(importHeaders, "|") {
		t.Fatalf("header = %v", raw[0])
	}
	rows, err := parseImportRows(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Row != 2 || rows[0].Username != "zhangsan" || strings.Join(rows[0].RoleCodes, ",") != "employee,approver" {
		t.Fatalf("example row = %+v", rows)
	}
	if msg := validateImportRow(rows[0]); msg != "" {
		t.Fatalf("example row must validate: %s", msg)
	}
}

func TestBuildExport(t *testing.T) {
	phone, dept := "13800000000", "技术部"
	last := time.Date(2026, 9, 9, 8, 0, 0, 0, time.Local)
	users := []User{{
		Username: "alice", Name: "爱丽丝", Phone: &phone, DeptName: &dept, Status: "locked",
		LastLoginAt: &last, CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local),
		Roles: []RoleBrief{{Code: "employee", Name: "员工"}, {Code: "finance", Name: "财务"}},
	}}
	f, err := buildExport(users)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	g, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := g.GetRows(g.GetSheetList()[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2 || strings.Join(raw[0], "|") != strings.Join(exportHeaders, "|") {
		t.Fatalf("rows = %v", raw)
	}
	want := []string{"alice", "爱丽丝", "13800000000", "", "", "技术部", "员工、财务", "锁定", "2026-09-09 08:00:00", "2026-01-02 03:04:05"}
	if strings.Join(raw[1], "|") != strings.Join(want, "|") {
		t.Fatalf("row = %v\nwant %v", raw[1], want)
	}
}
