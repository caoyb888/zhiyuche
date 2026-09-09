package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// RequireAuth validates the bearer token and attaches the Principal.
func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c)
		if raw == "" {
			httpx.Fail(c, httpx.Unauthorized("missing bearer token"))
			return
		}
		claims, err := parseAccess(s.app.Cfg.JWT.Secret, raw)
		if err != nil {
			httpx.Fail(c, httpx.Unauthorized("invalid or expired token"))
			return
		}
		ctx := c.Request.Context()
		if denied, err := s.tokens.isDenied(ctx, claims.ID); err != nil {
			httpx.Fail(c, httpx.Internal(err))
			return
		} else if denied {
			httpx.Fail(c, httpx.Unauthorized("token revoked"))
			return
		}
		uid, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.Fail(c, httpx.Unauthorized("invalid token subject"))
			return
		}
		cu, err := s.snapshot(ctx, uid)
		if err != nil {
			httpx.Fail(c, httpx.Internal(err))
			return
		}
		if cu == nil {
			httpx.Fail(c, httpx.Unauthorized("user not found"))
			return
		}
		if cu.Status != "active" {
			httpx.Fail(c, httpx.Forbidden("账号已停用或锁定"))
			return
		}
		p := &Principal{
			UserID:   uid,
			TenantID: cu.TenantID,
			Username: cu.Username,
			IsSuper:  cu.IsSuper,
			TokenID:  claims.ID,
			perms:    make(map[string]struct{}, len(cu.Perms)),
		}
		for _, code := range cu.Perms {
			p.perms[code] = struct{}{}
		}
		setPrincipal(c, p)
		c.Next()
	}
}

// Require returns a middleware that demands at least one of the given permission codes.
// It must run after RequireAuth.
func Require(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := Current(c)
		if p == nil {
			httpx.Fail(c, httpx.Unauthorized("unauthenticated"))
			return
		}
		for _, code := range codes {
			if p.Has(code) {
				c.Next()
				return
			}
		}
		httpx.Fail(c, httpx.Forbidden("没有权限："+strings.Join(codes, " / ")))
	}
}

// RequireSuper only lets platform super admins through.
func RequireSuper() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p := Current(c); p == nil || !p.IsSuper {
			httpx.Fail(c, httpx.Forbidden("仅平台管理员可操作"))
			return
		}
		c.Next()
	}
}

func bearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	// WebSocket / download links cannot set headers
	return c.Query("access_token")
}
