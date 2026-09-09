package card

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

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Card], error) {
	rows, total, err := s.store.list(ctx, tenantID, q, pg)
	if err != nil {
		return pagination.Page[Card]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Card, error) {
	c, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, httpx.NotFound("卡不存在")
	}
	return c, nil
}

func (s *Service) checkHolder(ctx context.Context, tenantID, userID uuid.UUID) error {
	ok, err := s.store.userActive(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.BadRequest("持卡人不存在或已离职")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest) (*Card, error) {
	uid, err := NormalizeUID(req.CardUID)
	if err != nil {
		return nil, err
	}
	if exists, err := s.store.uidExists(ctx, tenantID, uid); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("卡号已存在")
	}
	if req.UserID != nil {
		if err := s.checkHolder(ctx, tenantID, *req.UserID); err != nil {
			return nil, err
		}
	}
	id, err := s.store.create(ctx, tenantID, uid, req)
	if err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) (*Card, *Card, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.update(ctx, tenantID, id, req); err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Bind(ctx context.Context, tenantID, id, userID uuid.UUID) (*Card, *Card, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkHolder(ctx, tenantID, userID); err != nil {
		return nil, nil, err
	}
	if err := s.store.setUser(ctx, tenantID, id, &userID); err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Unbind(ctx context.Context, tenantID, id uuid.UUID) (*Card, *Card, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.setUser(ctx, tenantID, id, nil); err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

// ReportLoss marks the card lost; the trip module rejects lost cards at the vehicle.
func (s *Service) ReportLoss(ctx context.Context, tenantID, id uuid.UUID) (*Card, *Card, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.setStatus(ctx, tenantID, id, Lost); err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) (*Card, error) {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.store.softDelete(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return c, nil
}

func translateDBErr(err error) error {
	switch common.ConflictConstraint(err) {
	case "":
		return err
	case "uq_nfc_cards_uid":
		return httpx.Conflict("卡号已存在")
	default:
		return httpx.Conflict("数据冲突：" + common.ConflictConstraint(err))
	}
}
