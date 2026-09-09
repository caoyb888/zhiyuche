package role

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
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

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Role], error) {
	platform, err := s.store.isPlatform(ctx, tenantID)
	if err != nil {
		return pagination.Page[Role]{}, err
	}
	rows, total, err := s.store.list(ctx, tenantID, platform, q, pg)
	if err != nil {
		return pagination.Page[Role]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Role, error) {
	platform, err := s.store.isPlatform(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	r, err := s.store.get(ctx, tenantID, platform, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("角色不存在")
	}
	return r, nil
}

func (s *Service) Options(ctx context.Context, tenantID uuid.UUID) ([]Brief, error) {
	rows, err := s.store.options(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Brief{}
	}
	return rows, nil
}

// PermissionTree returns the registry tree visible to the tenant.
func (s *Service) PermissionTree(ctx context.Context, tenantID uuid.UUID) ([]*PermissionNode, error) {
	platform, err := s.store.isPlatform(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return BuildPermissionTree(platform), nil
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest) (*Role, error) {
	if err := ValidateCode(req.Code); err != nil {
		return nil, err
	}
	platform, err := s.store.isPlatform(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	perms, err := ValidatePermissions(req.Permissions, platform)
	if err != nil {
		return nil, err
	}
	if exists, err := s.store.codeExists(ctx, tenantID, req.Code); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("角色代码已存在")
	}
	id, err := s.store.create(ctx, tenantID, req, perms)
	if err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, tenantID, id)
}

// Update changes name/description only — for built-in and custom roles alike.
func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) (*Role, *Role, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.update(ctx, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) (*Role, error) {
	r, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r.IsSystem {
		return nil, httpx.Conflict("内置角色不能删除")
	}
	if r.UserCount > 0 {
		return nil, httpx.Conflict("仍有用户持有该角色，无法删除")
	}
	if err := s.store.softDelete(ctx, id); err != nil {
		return nil, err
	}
	_ = s.auth.InvalidateRole(ctx, id)
	return r, nil
}

// SetPermissions replaces the role's permission set and drops the cached
// snapshot of every holder so the change applies to their next request.
func (s *Service) SetPermissions(ctx context.Context, tenantID, id uuid.UUID, codes []string) (*Role, *Role, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	platform, err := s.store.isPlatform(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	// 平台级角色（tenant_id NULL）只在平台租户下可见，允许平台专属权限
	perms, err := ValidatePermissions(codes, platform || before.TenantID == nil)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.setPerms(ctx, id, perms); err != nil {
		return nil, nil, translateDBErr(err)
	}
	if err := s.auth.InvalidateRole(ctx, id); err != nil {
		s.app.Log.Warn().Err(err).Str("role_id", id.String()).Msg("invalidate role holders failed")
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

// translateDBErr maps unique-violation errors to 409.
func translateDBErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if pgErr.ConstraintName == "uq_roles_tenant_code" {
			return httpx.Conflict("角色代码已存在")
		}
		return httpx.Conflict("数据冲突：" + pgErr.ConstraintName)
	}
	return err
}
