package auditlog

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func TestParseFilter(t *testing.T) {
	uid := uuid.New()
	f, err := parseFilter(ListQuery{UserID: uid.String(), Username: " ad ", From: "2026-09-01T00:00:00Z", To: "2026-09-02", Keyword: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if f.UserID == nil || *f.UserID != uid || f.Username != "ad" || f.Keyword != "x" {
		t.Fatalf("unexpected filter: %+v", f)
	}
	if f.From == nil || !f.From.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %v", f.From)
	}
	if f.To == nil || f.To.Year() != 2026 || f.To.Month() != 9 || f.To.Day() != 2 {
		t.Fatalf("to = %v", f.To)
	}

	var ae *httpx.AppError
	if _, err := parseFilter(ListQuery{UserID: "nope"}); !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("bad user_id should be 400, got %v", err)
	}
	if _, err := parseFilter(ListQuery{From: "yesterday"}); !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("bad from should be 400, got %v", err)
	}
	if _, err := parseFilter(ListQuery{From: "2026-09-02T00:00:00Z", To: "2026-09-01T00:00:00Z"}); !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("to < from should be 400, got %v", err)
	}
	if f, err := parseFilter(ListQuery{}); err != nil || f.UserID != nil || f.From != nil || f.To != nil {
		t.Fatalf("empty query: %+v %v", f, err)
	}
}

func TestTenantFilter(t *testing.T) {
	platform, other := uuid.New(), uuid.New()
	super := &auth.Principal{IsSuper: true, TenantID: platform}
	if tf := tenantFilter(common.Resolve(super, platform)); tf != nil {
		t.Fatalf("super without header must see all tenants, got %v", tf)
	}
	if tf := tenantFilter(common.Resolve(super, other)); tf == nil || *tf != other {
		t.Fatalf("super with header must be limited to that tenant, got %v", tf)
	}
	if tf := tenantFilter(common.Resolve(&auth.Principal{TenantID: other}, other)); tf == nil || *tf != other {
		t.Fatalf("normal user must be limited to own tenant, got %v", tf)
	}
}

func TestListItemsRenderNullBeforeAfter(t *testing.T) {
	b, err := json.Marshal(AuditLog{ID: 1, Action: "create", Module: "system.user"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if v, ok := m["before"]; !ok || v != nil {
		t.Fatalf("before should be present and null, got %v", m["before"])
	}
	if v, ok := m["after"]; !ok || v != nil {
		t.Fatalf("after should be present and null, got %v", m["after"])
	}
}
