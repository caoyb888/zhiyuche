package template

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	store *store
}

func NewService(a *app.App) *Service { return &Service{store: &store{db: a.DB}} }

func (s *Service) List(ctx context.Context, sc common.Scope, q ListQuery, pg pagination.Query) (pagination.Page[Template], error) {
	rows, total, err := s.store.list(ctx, sc.TenantID, q, pg)
	if err != nil {
		return pagination.Page[Template]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, sc common.Scope, id uuid.UUID) (*Template, error) {
	t, err := s.store.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil || !sc.CanRead(t.TenantID) {
		return nil, httpx.NotFound("通知模板不存在")
	}
	return t, nil
}

func (s *Service) Create(ctx context.Context, sc common.Scope, req CreateRequest) (*Template, error) {
	req.Code = strings.TrimSpace(req.Code)
	if !ValidCode(req.Code) {
		return nil, httpx.BadRequest("模板代码须匹配 ^[a-z][a-z0-9_.]{1,63}$")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	id, err := s.store.create(ctx, sc.WriteTenant(), req, enabled)
	if err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, sc, id)
}

func (s *Service) Update(ctx context.Context, sc common.Scope, id uuid.UUID, req UpdateRequest) (*Template, *Template, error) {
	before, err := s.Get(ctx, sc, id)
	if err != nil {
		return nil, nil, err
	}
	if err := sc.CheckWrite(before.TenantID, "通知模板不存在"); err != nil {
		return nil, nil, err
	}
	if err := s.store.update(ctx, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, sc, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, sc common.Scope, id uuid.UUID) (*Template, error) {
	t, err := s.Get(ctx, sc, id)
	if err != nil {
		return nil, err
	}
	if err := sc.CheckWrite(t.TenantID, "通知模板不存在"); err != nil {
		return nil, err
	}
	if err := s.store.delete(ctx, id); err != nil {
		return nil, err
	}
	return t, nil
}

func translateDBErr(err error) error {
	switch cn := common.ConflictConstraint(err); cn {
	case "":
		return err
	case "uq_notification_templates":
		return httpx.Conflict("相同代码与渠道的模板已存在")
	default:
		return httpx.Conflict("数据冲突：" + cn)
	}
}
