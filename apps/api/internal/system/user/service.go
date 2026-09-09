package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/param"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app   *app.App
	store *store
	auth  *auth.Service
}

func NewService(a *app.App) *Service {
	return &Service{app: a, store: &store{db: a.DB}, auth: auth.NewService(a)}
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[User], error) {
	rows, total, err := s.store.list(ctx, tenantID, q, pg)
	if err != nil {
		return pagination.Page[User]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Options(ctx context.Context, tenantID uuid.UUID, q OptionsQuery) ([]UserOption, error) {
	return s.store.options(ctx, tenantID, q)
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*User, error) {
	u, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, httpx.NotFound("用户不存在")
	}
	return u, nil
}

// defaultPassword reads security.default_password (tenant override → global → built-in).
func (s *Service) defaultPassword(ctx context.Context, tenantID uuid.UUID) string {
	return param.Get(ctx, s.app.DB, tenantID, param.KeyDefaultPassword, "Zy@123456")
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest, actor *auth.Principal) (*User, error) {
	if exists, err := s.store.usernameExists(ctx, tenantID, req.Username); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("用户名已存在")
	}
	if req.Phone != nil && *req.Phone != "" {
		if exists, err := s.store.phoneExists(ctx, tenantID, *req.Phone, uuid.Nil); err != nil {
			return nil, err
		} else if exists {
			return nil, httpx.Conflict("手机号已被使用")
		}
	}
	if req.DeptID != nil {
		if ok, err := s.store.deptExists(ctx, tenantID, *req.DeptID); err != nil {
			return nil, err
		} else if !ok {
			return nil, httpx.BadRequest("部门不存在")
		}
	}
	pw := req.Password
	if pw == "" {
		pw = s.defaultPassword(ctx, tenantID)
	} else if err := auth.ValidatePassword(pw); err != nil {
		return nil, httpx.BadRequest(err.Error())
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return nil, err
	}
	roleIDs, err := s.store.validRoleIDs(ctx, tenantID, req.RoleIDs)
	if err != nil {
		return nil, err
	}
	if len(roleIDs) != len(dedupe(req.RoleIDs)) {
		return nil, httpx.BadRequest("包含不存在或不属于本租户的角色")
	}
	id, err := s.store.create(ctx, tenantID, req, hash, roleIDs, actor.UserID)
	if err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest, actor *auth.Principal) (*User, *User, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if before.IsSuper && !actor.IsSuper {
		return nil, nil, httpx.Forbidden("不能修改平台管理员")
	}
	if req.Status != nil && *req.Status != "active" && id == actor.UserID {
		return nil, nil, httpx.BadRequest("不能停用自己的账号")
	}
	if req.Phone != nil && *req.Phone != "" {
		if exists, err := s.store.phoneExists(ctx, tenantID, *req.Phone, id); err != nil {
			return nil, nil, err
		} else if exists {
			return nil, nil, httpx.Conflict("手机号已被使用")
		}
	}
	if req.DeptID != nil && !req.ClearDept {
		if ok, err := s.store.deptExists(ctx, tenantID, *req.DeptID); err != nil {
			return nil, nil, err
		} else if !ok {
			return nil, nil, httpx.BadRequest("部门不存在")
		}
	}
	if err := s.store.update(ctx, tenantID, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	if req.Status != nil && *req.Status != "active" {
		_ = s.auth.RevokeUser(ctx, id)
	} else {
		_ = s.auth.InvalidateUsers(ctx, id)
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID, actor *auth.Principal) (*User, error) {
	u, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if id == actor.UserID {
		return nil, httpx.BadRequest("不能删除自己的账号")
	}
	if u.IsSuper {
		return nil, httpx.Forbidden("不能删除平台管理员")
	}
	if err := s.store.softDelete(ctx, tenantID, id); err != nil {
		return nil, err
	}
	_ = s.auth.RevokeUser(ctx, id)
	return u, nil
}

func (s *Service) ResetPassword(ctx context.Context, tenantID, id uuid.UUID, req ResetPasswordRequest, actor *auth.Principal) (string, error) {
	u, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	if u.IsSuper && !actor.IsSuper {
		return "", httpx.Forbidden("不能重置平台管理员密码")
	}
	pw := req.Password
	if pw == "" {
		pw = s.defaultPassword(ctx, tenantID)
	} else if err := auth.ValidatePassword(pw); err != nil {
		return "", httpx.BadRequest(err.Error())
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return "", err
	}
	if err := s.store.setPassword(ctx, tenantID, id, hash); err != nil {
		return "", err
	}
	_ = s.auth.RevokeUser(ctx, id)
	return pw, nil
}

func (s *Service) AssignRoles(ctx context.Context, tenantID, id uuid.UUID, roleIDs []uuid.UUID, actor *auth.Principal) (*User, *User, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	valid, err := s.store.validRoleIDs(ctx, tenantID, roleIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(valid) != len(dedupe(roleIDs)) {
		return nil, nil, httpx.BadRequest("包含不存在或不属于本租户的角色")
	}
	if err := s.store.setRoles(ctx, id, valid); err != nil {
		return nil, nil, err
	}
	_ = s.auth.InvalidateUsers(ctx, id)
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	out := ids[:0:0]
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// translateDBErr maps unique-violation errors to 409.
func translateDBErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_users_tenant_username":
			return httpx.Conflict("用户名已存在")
		case "uq_users_tenant_phone":
			return httpx.Conflict("手机号已被使用")
		}
		return httpx.Conflict("数据冲突：" + pgErr.ConstraintName)
	}
	return err
}
