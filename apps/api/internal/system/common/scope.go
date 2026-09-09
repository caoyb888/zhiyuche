// Package common holds helpers shared by the "global + tenant" system modules
// (dict, param, template). Their tables keep platform-wide rows with
// tenant_id NULL that every tenant sees; a tenant row with the same key
// overrides the global one for that tenant only.
package common

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Scope is the resolved read/write target of a request.
type Scope struct {
	// TenantID is the tenant whose rows are read alongside the global ones:
	// the caller's own tenant, or X-Tenant-ID for a super admin.
	TenantID uuid.UUID
	// Global is true when writes target the global rows (tenant_id NULL):
	// a super admin acting without X-Tenant-ID.
	Global bool
	// IsSuper mirrors the principal; only super admins may touch global rows.
	IsSuper bool
}

// Resolve derives the scope from the principal and the effective tenant id
// (auth.TenantID). A super admin whose effective tenant is still its own
// platform tenant did not ask for a specific tenant, so it writes globally.
func Resolve(p *auth.Principal, tenantID uuid.UUID) Scope {
	s := Scope{TenantID: tenantID}
	if p == nil {
		return s
	}
	s.IsSuper = p.IsSuper
	s.Global = p.IsSuper && tenantID == p.TenantID
	return s
}

// FromContext resolves the scope of the current request.
func FromContext(c *gin.Context) Scope { return Resolve(auth.Current(c), auth.TenantID(c)) }

// WriteTenant is the tenant_id stored on a newly created row: nil for global.
func (s Scope) WriteTenant() *uuid.UUID {
	if s.Global {
		return nil
	}
	t := s.TenantID
	return &t
}

// CanRead reports whether a row (rowTenant nil = global) is visible to the caller.
func (s Scope) CanRead(rowTenant *uuid.UUID) bool {
	return rowTenant == nil || *rowTenant == s.TenantID
}

// CanWrite reports whether the caller may modify or delete a row:
// global rows only by super admins, tenant rows only by that tenant.
func (s Scope) CanWrite(rowTenant *uuid.UUID) bool {
	if rowTenant == nil {
		return s.IsSuper
	}
	return *rowTenant == s.TenantID
}

// CheckWrite turns the visibility / ownership checks into errors: 404 when
// the row is not visible at all, 403 when it is a global row and the caller
// is not a super admin.
func (s Scope) CheckWrite(rowTenant *uuid.UUID, notFoundMsg string) error {
	if !s.CanRead(rowTenant) {
		return httpx.NotFound(notFoundMsg)
	}
	if !s.CanWrite(rowTenant) {
		return httpx.Forbidden("全局数据仅平台管理员可修改")
	}
	return nil
}

// ConflictConstraint returns the violated unique constraint name when err is
// a PostgreSQL unique violation (23505), otherwise "".
func ConflictConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName
	}
	return ""
}
