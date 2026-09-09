package booking

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateWindow(t *testing.T) {
	now := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantErr bool
	}{
		{"正常时段", now.Add(time.Hour), now.Add(3 * time.Hour), false},
		{"允许补录过去的预约", now.Add(-48 * time.Hour), now.Add(-46 * time.Hour), false},
		{"结束早于开始", now.Add(3 * time.Hour), now.Add(time.Hour), true},
		{"起止相同", now, now, true},
		{"短于最小时长", now, now.Add(MinDuration - time.Minute), true},
		{"超过最大时长", now, now.Add(MaxDuration + time.Minute), true},
		{"补录过久", now.Add(-PastLimit - time.Hour), now.Add(-PastLimit), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWindow(c.start, c.end, now)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateWindow(%v, %v) err = %v, wantErr %v", c.start, c.end, err, c.wantErr)
			}
		})
	}
}

func TestNextStatus(t *testing.T) {
	ok := []struct {
		from  string
		event string
		want  string
	}{
		{StatusReserved, EvDepart, StatusDeparted},
		{StatusDeparted, EvComplete, StatusCompleted},
		{StatusReserved, EvCancel, StatusCancelled},
	}
	for _, c := range ok {
		got, err := NextStatus(c.from, c.event)
		if err != nil || got != c.want {
			t.Fatalf("NextStatus(%s, %s) = %q, %v; want %q", c.from, c.event, got, err, c.want)
		}
	}
	bad := []struct {
		from  string
		event string
	}{
		{StatusReserved, EvComplete}, // 未出车不能直接完成
		{StatusDeparted, EvCancel},   // 已出车只能完成
		{StatusDeparted, EvDepart},   // 重复出车
		{StatusCompleted, EvCancel},  // 终态
		{StatusCancelled, EvDepart},  // 终态
		{StatusReserved, "unknown"},  // 未知事件
	}
	for _, c := range bad {
		if _, err := NextStatus(c.from, c.event); !errors.Is(err, ErrTransition) {
			t.Fatalf("NextStatus(%s, %s) err = %v; want ErrTransition", c.from, c.event, err)
		}
	}
}

func TestEditable(t *testing.T) {
	if !Editable(StatusReserved) {
		t.Fatal("reserved should be editable")
	}
	for _, s := range []string{StatusDeparted, StatusCompleted, StatusCancelled} {
		if Editable(s) {
			t.Fatalf("%s should not be editable", s)
		}
	}
}

func TestCanDepart(t *testing.T) {
	for _, s := range []string{"idle", "charging"} {
		if !CanDepart(s) {
			t.Fatalf("%s should be allowed to depart", s)
		}
	}
	for _, s := range []string{"in_use", "maintenance", "disabled"} {
		if CanDepart(s) {
			t.Fatalf("%s should not be allowed to depart", s)
		}
	}
	if got, want := VehicleStatusLabel("maintenance"), "维保中"; got != want {
		t.Fatalf("VehicleStatusLabel = %q, want %q", got, want)
	}
	if got := VehicleStatusLabel("unknown"); got != "unknown" {
		t.Fatalf("VehicleStatusLabel fallback = %q, want raw code", got)
	}
}

func TestFormatWindow(t *testing.T) {
	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)   // 08:00 CST
	sameDay := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC) // 12:00 CST
	if got, want := FormatWindow(start, sameDay), "09-10 08:00 → 12:00"; got != want {
		t.Fatalf("FormatWindow same day = %q, want %q", got, want)
	}
	nextDay := time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC) // 09:00 CST 次日
	if got, want := FormatWindow(start, nextDay), "09-10 08:00 → 09-11 09:00"; got != want {
		t.Fatalf("FormatWindow cross day = %q, want %q", got, want)
	}
}

func TestConflictMessage(t *testing.T) {
	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)
	msg := ConflictMessage("京A12345", []Conflict{
		{Kind: "booking", No: "YY-20260910-0001", Who: "张伟", Start: start, End: end, Status: StatusReserved},
		{Kind: "approval", No: "ZY-20260910-0007", Who: "李明", Start: start, End: end, Status: "approved"},
	})
	for _, want := range []string{"京A12345", "预约 YY-20260910-0001", "申请 ZY-20260910-0007", "09-10 08:00 → 12:00"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("ConflictMessage = %q, missing %q", msg, want)
		}
	}
}
