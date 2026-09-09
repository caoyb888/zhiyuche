package tenant

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/bootstrap"
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

func (s *Service) List(ctx context.Context, q ListQuery, pg pagination.Query) (pagination.Page[Tenant], error) {
	rows, total, err := s.store.list(ctx, q, pg)
	if err != nil {
		return pagination.Page[Tenant]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	t, err := s.store.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, httpx.NotFound("租户不存在")
	}
	return t, nil
}

// Create inserts the tenant, seeds its built-in roles and creates the tenant
// administrator. The bootstrap helpers run outside a transaction, so a failure
// after the insert discards the half-created tenant instead of leaving it behind.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Tenant, error) {
	req = Normalize(req)
	if err := ValidateCode(req.Code); err != nil {
		return nil, err
	}
	pw := req.AdminPassword
	if pw == "" {
		// 全局缺省密码（新租户尚无覆盖值）
		pw = param.Get(ctx, s.app.DB, uuid.Nil, param.KeyDefaultPassword, "Zy@123456")
	} else if err := auth.ValidatePassword(pw); err != nil {
		return nil, httpx.BadRequest(err.Error())
	}
	if exists, err := s.store.codeExists(ctx, req.Code); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("租户代码已存在")
	}
	id, err := s.store.create(ctx, req)
	if err != nil {
		return nil, translateDBErr(err)
	}
	if err := bootstrap.EnsureTenantDefaults(ctx, s.app.DB, id); err != nil {
		s.undo(ctx, id, err)
		return nil, fmt.Errorf("seed tenant roles: %w", err)
	}
	if err := bootstrap.EnsureTenantAdmin(ctx, s.app.DB, id, req.AdminUsername, req.AdminName, pw, s.app.Log); err != nil {
		s.undo(ctx, id, err)
		return nil, fmt.Errorf("create tenant admin: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) undo(ctx context.Context, id uuid.UUID, cause error) {
	if err := s.store.discard(ctx, id); err != nil {
		s.app.Log.Error().Err(err).Str("tenant_id", id.String()).AnErr("cause", cause).Msg("discard half-created tenant failed")
	}
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*Tenant, *Tenant, error) {
	before, err := s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	tr, err := StatusTransition(before.Status, req.Status, before.IsPlatform)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.update(ctx, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	switch tr {
	case Disable:
		// 停用：该租户全部用户会话失效（refresh token 吊销 + 权限快照失效）
		if err := s.auth.RevokeTenant(ctx, id); err != nil {
			s.app.Log.Warn().Err(err).Str("tenant_id", id.String()).Msg("revoke tenant sessions failed")
		}
	case Reactivate:
		if err := s.auth.InvalidateTenant(ctx, id); err != nil {
			s.app.Log.Warn().Err(err).Str("tenant_id", id.String()).Msg("invalidate tenant snapshots failed")
		}
	}
	after, err := s.Get(ctx, id)
	return before, after, err
}

// Disable is DELETE /system/tenants/{id}: status → disabled, nothing is removed.
func (s *Service) Disable(ctx context.Context, id uuid.UUID) (*Tenant, *Tenant, error) {
	st := StatusDisabled
	return s.Update(ctx, id, UpdateRequest{Status: &st})
}

// translateDBErr maps unique-violation errors to 409.
func translateDBErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if pgErr.ConstraintName == "tenants_code_key" {
			return httpx.Conflict("租户代码已存在")
		}
		return httpx.Conflict("数据冲突：" + pgErr.ConstraintName)
	}
	return err
}
