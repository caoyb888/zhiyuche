package pile

import (
	"context"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app   *app.App
	store *store
}

func NewService(a *app.App) *Service { return &Service{app: a, store: &store{db: a.DB}} }

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Pile], error) {
	rows, total, err := s.store.list(ctx, tenantID, q, pg)
	if err != nil {
		return pagination.Page[Pile]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Pile, error) {
	p, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, httpx.NotFound("充电桩不存在")
	}
	return p, nil
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest) (*Pile, error) {
	code, err := NormalizeCode(req.PileCode)
	if err != nil {
		return nil, err
	}
	if exists, err := s.store.codeExists(ctx, tenantID, code); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("桩编号已存在")
	}
	id, err := s.store.create(ctx, tenantID, code, req)
	if err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) (*Pile, *Pile, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if req.Status != nil {
		if err := CheckManualStatus(*req.Status); err != nil {
			return nil, nil, err
		}
	}
	if err := s.store.update(ctx, tenantID, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) (*Pile, error) {
	p, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.store.softDelete(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return p, nil
}

func translateDBErr(err error) error {
	switch common.ConflictConstraint(err) {
	case "":
		return err
	case "uq_charge_piles_code":
		return httpx.Conflict("桩编号已存在")
	default:
		return httpx.Conflict("数据冲突：" + common.ConflictConstraint(err))
	}
}
