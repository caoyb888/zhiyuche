package auth

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// RegisterPublic mounts login/refresh (no auth).
func (s *Service) RegisterPublic(g *gin.RouterGroup) {
	g.POST("/auth/login", s.login)
	g.POST("/auth/refresh", s.refresh)
}

// RegisterProtected mounts the endpoints that need a principal.
func (s *Service) RegisterProtected(g *gin.RouterGroup) {
	g.POST("/auth/logout", s.logout)
	g.GET("/auth/me", s.me)
	g.GET("/auth/menus", s.menus)
	g.PUT("/auth/password", s.changePassword)
}

type loginRequest struct {
	Username   string `json:"username" binding:"required,min=2,max=64"`
	Password   string `json:"password" binding:"required,min=1,max=72"`
	TenantCode string `json:"tenant_code" binding:"max=64"`
}

type loginResponse struct {
	TokenPair
	User *Profile `json:"user"`
}

func (s *Service) login(c *gin.Context) {
	var req loginRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	ctx := c.Request.Context()
	ip := c.ClientIP()

	if locked, err := s.tokens.isLocked(ctx, req.Username, ip); err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	} else if locked {
		httpx.Fail(c, httpx.Forbidden("登录失败次数过多，请 15 分钟后再试"))
		return
	}

	u, err := s.findLoginUser(ctx, req.Username, req.TenantCode)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if u == nil || !VerifyPassword(u.PasswordHash, req.Password) {
		locked, _ := s.tokens.recordFailure(ctx, req.Username, ip)
		audit.Record(c, audit.Entry{Module: "auth", Action: "login_failed", ActorName: req.Username, Summary: "用户名或密码错误"})
		if locked {
			httpx.Fail(c, httpx.Forbidden("登录失败次数过多，账号已临时锁定 15 分钟"))
			return
		}
		httpx.Fail(c, httpx.Unauthorized("用户名或密码错误"))
		return
	}
	if u.Status != "active" {
		httpx.Fail(c, httpx.Forbidden("账号已停用或锁定，请联系管理员"))
		return
	}
	if u.TenantStatus != "active" {
		httpx.Fail(c, httpx.Forbidden("所属租户已停用"))
		return
	}

	pair, err := s.issuePair(ctx, u)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	prof, err := s.profile(ctx, u)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	s.tokens.clearFailures(ctx, req.Username, ip)
	s.touchLogin(ctx, u.ID, ip)
	audit.Record(c, audit.Entry{Module: "auth", Action: "login", ActorID: u.ID, ActorName: u.Username, TenantID: u.TenantID, Summary: "登录成功"})
	httpx.OK(c, loginResponse{TokenPair: *pair, User: prof})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (s *Service) refresh(c *gin.Context) {
	audit.Skip(c)
	var req refreshRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	ctx := c.Request.Context()
	rec, err := s.tokens.consumeRefresh(ctx, req.RefreshToken)
	if errors.Is(err, errRefreshInvalid) {
		httpx.Fail(c, httpx.Unauthorized("refresh token 无效或已过期"))
		return
	}
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	u, err := s.loadUser(ctx, rec.UserID)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	if u == nil || u.Status != "active" || u.TenantStatus != "active" {
		httpx.Fail(c, httpx.Forbidden("账号不可用"))
		return
	}
	pair, err := s.issuePair(ctx, u)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	httpx.OK(c, pair)
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Service) logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req) // body optional
	ctx := c.Request.Context()
	// deny the current access token for its remaining lifetime
	if claims, err := parseAccess(s.app.Cfg.JWT.Secret, bearer(c)); err == nil && claims.ExpiresAt != nil {
		_ = s.tokens.deny(ctx, claims.ID, claims.ExpiresAt.Time)
	}
	if req.RefreshToken != "" {
		_, _ = s.tokens.consumeRefresh(ctx, req.RefreshToken)
	}
	audit.Record(c, audit.Entry{Module: "auth", Action: "logout", Summary: "退出登录"})
	httpx.OK(c, gin.H{"logged_out_at": time.Now()})
}

func (s *Service) me(c *gin.Context) {
	p := Current(c)
	u, err := s.loadUser(c.Request.Context(), p.UserID)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	if u == nil {
		httpx.Fail(c, httpx.Unauthorized("user not found"))
		return
	}
	prof, err := s.profile(c.Request.Context(), u)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	httpx.OK(c, prof)
}

func (s *Service) menus(c *gin.Context) {
	p := Current(c)
	u, err := s.loadUser(c.Request.Context(), p.UserID)
	if err != nil || u == nil {
		httpx.Fail(c, httpx.Unauthorized("user not found"))
		return
	}
	prof, err := s.profile(c.Request.Context(), u)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	httpx.OK(c, prof.Menus)
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func (s *Service) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		httpx.Fail(c, httpx.BadRequest(err.Error()))
		return
	}
	p := Current(c)
	ctx := c.Request.Context()
	u, err := s.loadUser(ctx, p.UserID)
	if err != nil || u == nil {
		httpx.Fail(c, httpx.Unauthorized("user not found"))
		return
	}
	if !VerifyPassword(u.PasswordHash, req.OldPassword) {
		httpx.Fail(c, httpx.BadRequest("原密码不正确"))
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	if err := s.updatePassword(ctx, p.UserID, hash); err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	// other sessions must re-login; keep the current access token alive
	_ = s.tokens.revokeAllRefresh(ctx, p.UserID)
	audit.Record(c, audit.Entry{Module: "auth", Action: "change_password", TargetType: "user", TargetID: p.UserID.String(), Summary: "修改本人密码"})
	httpx.OK(c, gin.H{"changed": true})
}
