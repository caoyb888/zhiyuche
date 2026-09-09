package device

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/internal/telemetry"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app   *app.App
	store *store
}

func NewService(a *app.App) *Service { return &Service{app: a, store: &store{db: a.DB}} }

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Device], error) {
	rows, total, err := s.store.list(ctx, tenantID, q, pg)
	if err != nil {
		return pagination.Page[Device]{}, err
	}
	now := time.Now()
	for i := range rows {
		rows[i].Online = IsOnline(rows[i].LastOnlineAt, now)
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Device, error) {
	d, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, httpx.NotFound("设备不存在")
	}
	d.Online = IsOnline(d.LastOnlineAt, time.Now())
	return d, nil
}

// Create registers a gateway and returns its one-time api key.
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest) (*WithKey, error) {
	serial, err := NormalizeSerial(req.SerialNo)
	if err != nil {
		return nil, err
	}
	if exists, err := s.store.serialExists(ctx, serial); err != nil {
		return nil, err
	} else if exists {
		return nil, httpx.Conflict("序列号已存在")
	}
	if req.VehicleID != nil {
		v, err := s.store.vehicle(ctx, tenantID, *req.VehicleID)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, httpx.BadRequest("车辆不存在")
		}
		if v.DeviceID != nil {
			return nil, httpx.Conflict("目标车辆已绑定其他设备")
		}
	}
	key, hash, err := telemetry.NewAPIKey()
	if err != nil {
		return nil, err
	}
	id, err := s.store.create(ctx, tenantID, serial, hash, req)
	if err != nil {
		return nil, translateDBErr(err)
	}
	d, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return &WithKey{Device: *d, APIKey: key}, nil
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) (*Device, *Device, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.update(ctx, tenantID, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

// Bind attaches the device to a vehicle of the same tenant (one gateway per vehicle).
func (s *Service) Bind(ctx context.Context, tenantID, id, vehicleID uuid.UUID) (*Device, *Device, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	v, err := s.store.vehicle(ctx, tenantID, vehicleID)
	if err != nil {
		return nil, nil, err
	}
	if v == nil {
		return nil, nil, httpx.BadRequest("车辆不存在")
	}
	noop, err := CheckBind(uuidStr(before.VehicleID), uuidStr(&vehicleID), uuidStr(v.DeviceID), uuidStr(&id))
	if err != nil {
		return nil, nil, err
	}
	if noop {
		return before, before, nil
	}
	if err := s.store.setVehicle(ctx, tenantID, id, &vehicleID); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

// Unbind detaches the device; refused while the vehicle is on a trip.
func (s *Service) Unbind(ctx context.Context, tenantID, id uuid.UUID) (*Device, *Device, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if before.VehicleID == nil {
		return nil, nil, httpx.BadRequest("设备未绑定车辆")
	}
	if err := s.checkVehicleFree(ctx, tenantID, *before.VehicleID); err != nil {
		return nil, nil, err
	}
	if err := s.store.setVehicle(ctx, tenantID, id, nil); err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) checkVehicleFree(ctx context.Context, tenantID, vehicleID uuid.UUID) error {
	v, err := s.store.vehicle(ctx, tenantID, vehicleID)
	if err != nil {
		return err
	}
	if v == nil { // vehicle already gone: nothing to protect
		return nil
	}
	return CheckUnbind(v.Status, v.TripID != nil)
}

// RotateKey replaces the api key; the old one stops working immediately.
func (s *Service) RotateKey(ctx context.Context, tenantID, id uuid.UUID) (*WithKey, error) {
	if _, err := s.Get(ctx, tenantID, id); err != nil {
		return nil, err
	}
	key, hash, err := telemetry.NewAPIKey()
	if err != nil {
		return nil, err
	}
	if err := s.store.setKeyHash(ctx, tenantID, id, hash); err != nil {
		return nil, err
	}
	d, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return &WithKey{Device: *d, APIKey: key}, nil
}

// Delete soft-deletes the device (unbinding it); refused while its vehicle is on a trip.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) (*Device, error) {
	d, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if d.VehicleID != nil {
		if err := s.checkVehicleFree(ctx, tenantID, *d.VehicleID); err != nil {
			return nil, err
		}
	}
	if err := s.store.softDelete(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return d, nil
}

func uuidStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func translateDBErr(err error) error {
	switch common.ConflictConstraint(err) {
	case "":
		return err
	case "uq_devices_serial":
		return httpx.Conflict("序列号已存在")
	case "uq_devices_vehicle":
		return httpx.Conflict("目标车辆已绑定其他设备")
	default:
		return httpx.Conflict("数据冲突：" + common.ConflictConstraint(err))
	}
}
