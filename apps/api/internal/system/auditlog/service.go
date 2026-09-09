package auditlog

import (
	"context"

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

// tenantFilter: normal users and super admins with X-Tenant-ID see one
// tenant; a super admin without the header sees everything.
func tenantFilter(sc common.Scope) *uuid.UUID {
	if sc.Global {
		return nil
	}
	t := sc.TenantID
	return &t
}

func (s *Service) List(ctx context.Context, sc common.Scope, q ListQuery, pg pagination.Query) (pagination.Page[AuditLog], error) {
	f, err := parseFilter(q)
	if err != nil {
		return pagination.Page[AuditLog]{}, err
	}
	rows, total, err := s.store.list(ctx, tenantFilter(sc), f, pg)
	if err != nil {
		return pagination.Page[AuditLog]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) Get(ctx context.Context, sc common.Scope, id int64) (*AuditLog, error) {
	row, err := s.store.get(ctx, tenantFilter(sc), id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, httpx.NotFound("审计日志不存在")
	}
	return row, nil
}
