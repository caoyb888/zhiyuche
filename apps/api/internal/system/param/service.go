package param

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

type Service struct {
	store *store
}

func NewService(a *app.App) *Service { return &Service{store: &store{db: a.DB}} }

// List returns every global key with the tenant's effective value.
func (s *Service) List(ctx context.Context, sc common.Scope, q ListQuery) ([]Param, error) {
	return s.store.list(ctx, sc.TenantID, q)
}

func (s *Service) Get(ctx context.Context, sc common.Scope, key string) (*Param, error) {
	p, err := s.store.getEffective(ctx, sc.TenantID, key)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, httpx.NotFound("参数不存在")
	}
	return p, nil
}

// Set writes the value: the global default when the scope is global (super
// admin without X-Tenant-ID), otherwise the tenant override. Keys must already
// exist globally — the API never creates new keys.
func (s *Service) Set(ctx context.Context, sc common.Scope, key string, req UpdateRequest, actor uuid.UUID) (*Param, *Param, error) {
	key = strings.TrimSpace(key)
	g, err := s.store.getGlobal(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if g == nil {
		return nil, nil, httpx.NotFound("参数不存在")
	}
	value := *req.Value
	if err := ValidateValue(g.ValueType, value); err != nil {
		return nil, nil, httpx.BadRequest(err.Error())
	}
	before, err := s.Get(ctx, sc, key)
	if err != nil {
		return nil, nil, err
	}
	if sc.Global {
		err = s.store.setGlobal(ctx, key, value, req.Description, actor)
	} else {
		err = s.store.upsertTenant(ctx, sc.TenantID, key, value, req.Description, actor)
	}
	if err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, sc, key)
	return before, after, err
}

// Reset deletes the tenant override so the global default applies again.
func (s *Service) Reset(ctx context.Context, sc common.Scope, key string) (*Param, error) {
	key = strings.TrimSpace(key)
	before, err := s.Get(ctx, sc, key)
	if err != nil {
		return nil, err
	}
	if sc.Global {
		return nil, httpx.BadRequest("全局缺省值不可删除")
	}
	deleted, err := s.store.deleteTenant(ctx, sc.TenantID, key)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, httpx.NotFound("该参数没有租户覆盖值")
	}
	return before, nil
}
