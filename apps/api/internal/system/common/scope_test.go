package common

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func TestResolve(t *testing.T) {
	platform, other := uuid.New(), uuid.New()
	super := &auth.Principal{IsSuper: true, TenantID: platform}
	normal := &auth.Principal{TenantID: other}

	cases := []struct {
		name     string
		p        *auth.Principal
		tenant   uuid.UUID
		global   bool
		writeNil bool
	}{
		{"super without header writes global", super, platform, true, true},
		{"super with header writes that tenant", super, other, false, false},
		{"normal user writes own tenant", normal, other, false, false},
		{"anonymous", nil, other, false, false},
	}
	for _, tc := range cases {
		s := Resolve(tc.p, tc.tenant)
		if s.Global != tc.global {
			t.Errorf("%s: Global = %v, want %v", tc.name, s.Global, tc.global)
		}
		if (s.WriteTenant() == nil) != tc.writeNil {
			t.Errorf("%s: WriteTenant nil = %v, want %v", tc.name, s.WriteTenant() == nil, tc.writeNil)
		}
		if s.TenantID != tc.tenant {
			t.Errorf("%s: TenantID mismatch", tc.name)
		}
	}
}

func TestCanReadWrite(t *testing.T) {
	platform, mine, theirs := uuid.New(), uuid.New(), uuid.New()
	superScope := Resolve(&auth.Principal{IsSuper: true, TenantID: platform}, platform)
	superOnTenant := Resolve(&auth.Principal{IsSuper: true, TenantID: platform}, mine)
	normal := Resolve(&auth.Principal{TenantID: mine}, mine)

	if !normal.CanRead(nil) || !normal.CanRead(&mine) || normal.CanRead(&theirs) {
		t.Fatal("normal read rules wrong")
	}
	if normal.CanWrite(nil) || !normal.CanWrite(&mine) || normal.CanWrite(&theirs) {
		t.Fatal("normal write rules wrong")
	}
	if !superScope.CanWrite(nil) || !superOnTenant.CanWrite(nil) || !superOnTenant.CanWrite(&mine) || superOnTenant.CanWrite(&theirs) {
		t.Fatal("super write rules wrong")
	}

	var ae *httpx.AppError
	if err := normal.CheckWrite(&theirs, "nope"); !errors.As(err, &ae) || ae.Status != 404 {
		t.Fatalf("other tenant row should be 404, got %v", err)
	}
	if err := normal.CheckWrite(nil, "nope"); !errors.As(err, &ae) || ae.Status != 403 {
		t.Fatalf("global row for normal user should be 403, got %v", err)
	}
	if err := superScope.CheckWrite(nil, "nope"); err != nil {
		t.Fatalf("super may write global: %v", err)
	}
}

func TestConflictConstraint(t *testing.T) {
	if got := ConflictConstraint(&pgconn.PgError{Code: "23505", ConstraintName: "uq_x"}); got != "uq_x" {
		t.Fatalf("got %q", got)
	}
	if got := ConflictConstraint(&pgconn.PgError{Code: "23503"}); got != "" {
		t.Fatalf("fk violation must not be a conflict, got %q", got)
	}
	if got := ConflictConstraint(errors.New("plain")); got != "" {
		t.Fatalf("plain error, got %q", got)
	}
}
