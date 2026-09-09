package vehicle

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app   *app.App
	store *store
	vs    *vstatus.Store
}

func NewService(a *app.App) *Service {
	return &Service{app: a, store: &store{db: a.DB}, vs: vstatus.New(a)}
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Vehicle], error) {
	rows, total, err := s.store.list(ctx, tenantID, q, pg)
	if err != nil {
		return pagination.Page[Vehicle]{}, err
	}
	if err := s.attachLive(ctx, rows); err != nil {
		return pagination.Page[Vehicle]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) attachLive(ctx context.Context, rows []Vehicle) error {
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	live, err := s.vs.LiveByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range rows {
		rows[i].Live = live[rows[i].ID]
	}
	return nil
}

// Get returns the vehicle with its live snapshot and accumulated trip count.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Vehicle, error) {
	v, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, httpx.NotFound("车辆不存在")
	}
	if v.Live, err = s.vs.GetLive(ctx, id); err != nil {
		return nil, err
	}
	n, err := s.store.tripCount(ctx, id)
	if err != nil {
		return nil, err
	}
	v.TripCount = &n
	return v, nil
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest, actor *auth.Principal) (*Vehicle, error) {
	if exists, err := s.store.plateExists(ctx, tenantID, req.PlateNo, uuid.Nil); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("车牌号已存在")
	}
	if vin := nilIfEmpty(req.VIN); vin != nil {
		if exists, err := s.store.vinExists(ctx, *vin, uuid.Nil); err != nil {
			return nil, err
		} else if exists {
			return nil, httpx.Conflict("VIN 已存在")
		}
	}
	if req.HomeDeptID != nil {
		if ok, err := s.store.deptExists(ctx, tenantID, *req.HomeDeptID); err != nil {
			return nil, err
		} else if !ok {
			return nil, httpx.BadRequest("归属部门不存在")
		}
	}
	id, err := s.store.create(ctx, tenantID, req, actor.UserID)
	if err != nil {
		return nil, translateDBErr(err)
	}
	if err := s.vs.Ensure(ctx, tenantID, id, vstatus.Idle); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) (*Vehicle, *Vehicle, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if req.PlateNo != nil && *req.PlateNo != before.PlateNo {
		if exists, err := s.store.plateExists(ctx, tenantID, *req.PlateNo, id); err != nil {
			return nil, nil, err
		} else if exists {
			return nil, nil, httpx.Conflict("车牌号已存在")
		}
	}
	if vin := nilIfEmpty(req.VIN); vin != nil && (before.VIN == nil || *vin != *before.VIN) {
		if exists, err := s.store.vinExists(ctx, *vin, id); err != nil {
			return nil, nil, err
		} else if exists {
			return nil, nil, httpx.Conflict("VIN 已存在")
		}
	}
	if req.HomeDeptID != nil && !req.ClearHomeDept {
		if ok, err := s.store.deptExists(ctx, tenantID, *req.HomeDeptID); err != nil {
			return nil, nil, err
		} else if !ok {
			return nil, nil, httpx.BadRequest("归属部门不存在")
		}
	}
	if req.Status != nil {
		if err := CheckManualStatus(before.Status, hasTrip(before), *req.Status); err != nil {
			return nil, nil, err
		}
	}
	if err := s.store.update(ctx, tenantID, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	if req.Status != nil && *req.Status != before.Status {
		// 状态切换经 vstatus 写入并推送 vehicle.status
		if _, err := s.vs.SetStatus(ctx, id, *req.Status, nil, nil); err != nil {
			return nil, nil, err
		}
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) (*Vehicle, error) {
	v, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	n, err := s.store.activeApprovals(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := CheckDeletable(v.Status, hasTrip(v), n); err != nil {
		return nil, err
	}
	if err := s.store.softDelete(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) Options(ctx context.Context, tenantID uuid.UUID, status string) ([]Brief, error) {
	return s.store.options(ctx, tenantID, status)
}

func (s *Service) LiveAll(ctx context.Context, tenantID uuid.UUID) ([]vstatus.VehicleLive, error) {
	return s.vs.ListLiveByTenant(ctx, tenantID)
}

func (s *Service) Live(ctx context.Context, tenantID, id uuid.UUID) (*vstatus.VehicleLive, error) {
	live, err := s.vs.GetLive(ctx, id)
	if err != nil {
		return nil, err
	}
	if live == nil || live.TenantID != tenantID {
		return nil, httpx.NotFound("车辆不存在")
	}
	return live, nil
}

func (s *Service) Telemetry(ctx context.Context, tenantID, id uuid.UUID, from, to time.Time, limit int) ([]TelemetryPoint, error) {
	if err := CheckTelemetryRange(from, to); err != nil {
		return nil, err
	}
	v, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, httpx.NotFound("车辆不存在")
	}
	return s.store.telemetry(ctx, id, from, to, limit)
}

func hasTrip(v *Vehicle) bool { return v.Live != nil && v.Live.CurrentTripID != nil }

// translateDBErr maps unique violations to 409.
func translateDBErr(err error) error {
	switch common.ConflictConstraint(err) {
	case "":
		return err
	case "uq_vehicles_tenant_plate":
		return httpx.Conflict("车牌号已存在")
	case "uq_vehicles_vin":
		return httpx.Conflict("VIN 已存在")
	default:
		return httpx.Conflict("数据冲突：" + common.ConflictConstraint(err))
	}
}
