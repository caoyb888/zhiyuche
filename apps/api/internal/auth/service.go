package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/perm"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Service is the authentication facade used by handlers, the middleware and other
// modules (for cache invalidation after role/status changes).
type Service struct {
	app    *app.App
	tokens *tokenStore
}

func NewService(a *app.App) *Service {
	return &Service{app: a, tokens: &tokenStore{rdb: a.Redis}}
}

type dbUser struct {
	ID           uuid.UUID  `db:"id"`
	TenantID     uuid.UUID  `db:"tenant_id"`
	TenantCode   string     `db:"tenant_code"`
	TenantName   string     `db:"tenant_name"`
	TenantStatus string     `db:"tenant_status"`
	Username     string     `db:"username"`
	PasswordHash string     `db:"password_hash"`
	Name         string     `db:"name"`
	Status       string     `db:"status"`
	IsSuper      bool       `db:"is_super"`
	DeptID       *uuid.UUID `db:"dept_id"`
	DeptName     *string    `db:"dept_name"`
	Phone        *string    `db:"phone"`
	Email        *string    `db:"email"`
	AvatarURL    *string    `db:"avatar_url"`
	LastLoginAt  *time.Time `db:"last_login_at"`
}

const userSelect = `
	SELECT u.id, u.tenant_id, t.code AS tenant_code, t.name AS tenant_name, t.status AS tenant_status,
	       u.username, u.password_hash, u.name, u.status, u.is_super, u.dept_id, d.name AS dept_name,
	       u.phone, u.email, u.avatar_url, u.last_login_at
	FROM users u
	JOIN tenants t ON t.id = u.tenant_id
	LEFT JOIN departments d ON d.id = u.dept_id
	WHERE u.deleted_at IS NULL`

// findLoginUser resolves a username, optionally within a tenant code.
func (s *Service) findLoginUser(ctx context.Context, username, tenantCode string) (*dbUser, error) {
	var rows []dbUser
	q := userSelect + ` AND u.username = $1`
	args := []any{username}
	if tenantCode != "" {
		q += ` AND t.code = $2`
		args = append(args, tenantCode)
	}
	if err := pgxscan.Select(ctx, s.app.DB, &rows, q, args...); err != nil {
		return nil, err
	}
	switch len(rows) {
	case 0:
		return nil, nil
	case 1:
		return &rows[0], nil
	default:
		return nil, httpx.BadRequest("该用户名存在于多个租户，请指定租户代码")
	}
}

func (s *Service) loadUser(ctx context.Context, id uuid.UUID) (*dbUser, error) {
	var u dbUser
	err := pgxscan.Get(ctx, s.app.DB, &u, userSelect+` AND u.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s *Service) loadPerms(ctx context.Context, uid uuid.UUID) ([]string, error) {
	var codes []string
	err := pgxscan.Select(ctx, s.app.DB, &codes, `
		SELECT DISTINCT rp.permission_code
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		JOIN role_permissions rp ON rp.role_id = r.id
		WHERE ur.user_id = $1`, uid)
	return codes, err
}

// snapshot loads (or refreshes) the cached per-user record.
func (s *Service) snapshot(ctx context.Context, uid uuid.UUID) (*cachedUser, error) {
	if cu, err := s.tokens.getCachedUser(ctx, uid); err == nil && cu != nil {
		return cu, nil
	}
	u, err := s.loadUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}
	perms, err := s.loadPerms(ctx, uid)
	if err != nil {
		return nil, err
	}
	status := u.Status
	if u.TenantStatus != "active" {
		status = "tenant_disabled"
	}
	cu := &cachedUser{TenantID: u.TenantID, Username: u.Username, Status: status, IsSuper: u.IsSuper, Perms: perms}
	_ = s.tokens.setCachedUser(ctx, uid, cu)
	return cu, nil
}

// InvalidateUsers drops the cached snapshot of the given users (call after role/status/tenant changes).
func (s *Service) InvalidateUsers(ctx context.Context, ids ...uuid.UUID) error {
	return s.tokens.invalidatePerms(ctx, ids...)
}

// InvalidateRole drops the snapshot of every user holding the role.
func (s *Service) InvalidateRole(ctx context.Context, roleID uuid.UUID) error {
	var ids []uuid.UUID
	if err := pgxscan.Select(ctx, s.app.DB, &ids, `SELECT user_id FROM user_roles WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	return s.tokens.invalidatePerms(ctx, ids...)
}

// RevokeUser logs a user out everywhere: refresh tokens gone, snapshot dropped.
func (s *Service) RevokeUser(ctx context.Context, uid uuid.UUID) error {
	if err := s.tokens.revokeAllRefresh(ctx, uid); err != nil {
		return err
	}
	return s.tokens.invalidatePerms(ctx, uid)
}

// ---- token pair ----

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token"`
}

func (s *Service) issuePair(ctx context.Context, u *dbUser) (*TokenPair, error) {
	at, err := issueAccess(s.app.Cfg.JWT.Secret, s.app.Cfg.JWT.AccessTTL, u.ID, u.TenantID, u.Username, u.IsSuper)
	if err != nil {
		return nil, err
	}
	rt, err := s.tokens.issueRefresh(ctx, u.ID, u.TenantID, s.app.Cfg.JWT.RefreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: at.Token, ExpiresAt: at.ExpiresAt, RefreshToken: rt}, nil
}

// ---- profile ----

type Profile struct {
	ID          uuid.UUID        `json:"id"`
	Username    string           `json:"username"`
	Name        string           `json:"name"`
	Phone       *string          `json:"phone"`
	Email       *string          `json:"email"`
	AvatarURL   *string          `json:"avatar_url"`
	IsSuper     bool             `json:"is_super"`
	DeptID      *uuid.UUID       `json:"dept_id"`
	DeptName    *string          `json:"dept_name"`
	Tenant      TenantBrief      `json:"tenant"`
	Roles       []RoleBrief      `json:"roles"`
	Permissions []string         `json:"permissions"`
	Menus       []*perm.MenuNode `json:"menus"`
	LastLoginAt *time.Time       `json:"last_login_at"`
}

type TenantBrief struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

type RoleBrief struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

func (s *Service) profile(ctx context.Context, u *dbUser) (*Profile, error) {
	var roles []RoleBrief
	if err := pgxscan.Select(ctx, s.app.DB, &roles, `
		SELECT r.id, r.code, r.name FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE ur.user_id = $1 ORDER BY r.code`, u.ID); err != nil {
		return nil, err
	}
	perms, err := s.loadPerms(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, p := range perms {
		set[p] = struct{}{}
	}
	has := func(code string) bool { _, ok := set[code]; return ok }
	if u.IsSuper {
		perms = perm.ActionCodes(true)
		has = nil
	}
	if roles == nil {
		roles = []RoleBrief{}
	}
	if perms == nil {
		perms = []string{}
	}
	return &Profile{
		ID: u.ID, Username: u.Username, Name: u.Name, Phone: u.Phone, Email: u.Email, AvatarURL: u.AvatarURL,
		IsSuper: u.IsSuper, DeptID: u.DeptID, DeptName: u.DeptName,
		Tenant:      TenantBrief{ID: u.TenantID, Code: u.TenantCode, Name: u.TenantName},
		Roles:       roles,
		Permissions: perms,
		Menus:       orEmptyMenus(perm.MenusFor(has)),
		LastLoginAt: u.LastLoginAt,
	}, nil
}

func orEmptyMenus(m []*perm.MenuNode) []*perm.MenuNode {
	if m == nil {
		return []*perm.MenuNode{}
	}
	return m
}

func (s *Service) touchLogin(ctx context.Context, uid uuid.UUID, ip string) {
	_, err := s.app.DB.Exec(ctx, `UPDATE users SET last_login_at = now(), last_login_ip = $2 WHERE id = $1`, uid, ip)
	if err != nil {
		s.app.Log.Warn().Err(err).Msg("touch login failed")
	}
}

func (s *Service) updatePassword(ctx context.Context, uid uuid.UUID, hash string) error {
	_, err := s.app.DB.Exec(ctx, `UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1`, uid, hash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}
