// Package auth implements JWT authentication, refresh tokens, password handling,
// the request principal, and the RBAC middleware.
package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Principal is the authenticated caller attached to the request context.
type Principal struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Username string
	IsSuper  bool
	perms    map[string]struct{}
	TokenID  string
}

// Has reports whether the principal holds a permission code. Super admins hold everything.
func (p *Principal) Has(code string) bool {
	if p == nil {
		return false
	}
	if p.IsSuper {
		return true
	}
	_, ok := p.perms[code]
	return ok
}

// Perms returns the sorted-insensitive list of permission codes.
func (p *Principal) Perms() []string {
	out := make([]string, 0, len(p.perms))
	for k := range p.perms {
		out = append(out, k)
	}
	return out
}

const (
	ctxPrincipal = "auth.principal"
	// HeaderTenant lets a super admin act on another tenant.
	HeaderTenant = "X-Tenant-ID"
)

// Current returns the principal, or nil for anonymous requests.
func Current(c *gin.Context) *Principal {
	if v, ok := c.Get(ctxPrincipal); ok {
		if p, ok := v.(*Principal); ok {
			return p
		}
	}
	return nil
}

func setPrincipal(c *gin.Context, p *Principal) { c.Set(ctxPrincipal, p) }

// TenantID resolves the tenant the request operates on: the caller's own tenant,
// or the X-Tenant-ID header when the caller is a super admin.
func TenantID(c *gin.Context) uuid.UUID {
	p := Current(c)
	if p == nil {
		return uuid.Nil
	}
	if p.IsSuper {
		if h := c.GetHeader(HeaderTenant); h != "" {
			if id, err := uuid.Parse(h); err == nil {
				return id
			}
		}
	}
	return p.TenantID
}
