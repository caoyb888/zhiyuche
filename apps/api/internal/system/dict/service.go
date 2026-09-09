package dict

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

func (s *Service) List(ctx context.Context, sc common.Scope, q ListQuery, pg pagination.Query) (pagination.Page[DictType], error) {
	rows, total, err := s.store.list(ctx, sc.TenantID, q, pg)
	if err != nil {
		return pagination.Page[DictType]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

// visibleType loads a type the caller may see (global or own tenant).
func (s *Service) visibleType(ctx context.Context, sc common.Scope, id uuid.UUID) (*DictType, error) {
	t, err := s.store.getType(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil || !sc.CanRead(t.TenantID) {
		return nil, httpx.NotFound("字典类型不存在")
	}
	return t, nil
}

// Get returns the type with its items.
func (s *Service) Get(ctx context.Context, sc common.Scope, id uuid.UUID) (*DictType, error) {
	t, err := s.visibleType(ctx, sc, id)
	if err != nil {
		return nil, err
	}
	if t.Items, err = s.store.items(ctx, t.ID); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Create(ctx context.Context, sc common.Scope, req CreateRequest) (*DictType, error) {
	req.Code = strings.TrimSpace(req.Code)
	if !ValidCode(req.Code) {
		return nil, httpx.BadRequest("字典代码须匹配 ^[a-z][a-z0-9_]{1,63}$")
	}
	if err := checkItemValues(req.Items); err != nil {
		return nil, err
	}
	id, err := s.store.createType(ctx, sc.WriteTenant(), req)
	if err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, sc, id)
}

// Update edits name/description. Global types are only editable by super admins.
func (s *Service) Update(ctx context.Context, sc common.Scope, id uuid.UUID, req UpdateRequest) (*DictType, *DictType, error) {
	before, err := s.Get(ctx, sc, id)
	if err != nil {
		return nil, nil, err
	}
	if err := sc.CheckWrite(before.TenantID, "字典类型不存在"); err != nil {
		return nil, nil, err
	}
	if err := s.store.updateType(ctx, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, sc, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, sc common.Scope, id uuid.UUID) (*DictType, error) {
	t, err := s.Get(ctx, sc, id)
	if err != nil {
		return nil, err
	}
	if err := sc.CheckWrite(t.TenantID, "字典类型不存在"); err != nil {
		return nil, err
	}
	if t.IsSystem {
		return nil, httpx.Conflict("系统字典不可删除")
	}
	if err := s.store.deleteType(ctx, id); err != nil {
		return nil, err
	}
	return t, nil
}

// CreateItem adds an item to a type the caller owns.
func (s *Service) CreateItem(ctx context.Context, sc common.Scope, typeID uuid.UUID, req ItemCreateRequest) (*DictType, *DictItem, error) {
	t, err := s.visibleType(ctx, sc, typeID)
	if err != nil {
		return nil, nil, err
	}
	if err := sc.CheckWrite(t.TenantID, "字典类型不存在"); err != nil {
		return nil, nil, err
	}
	if err := checkItemValues([]ItemCreateRequest{req}); err != nil {
		return nil, nil, err
	}
	id, err := s.store.createItem(ctx, typeID, req)
	if err != nil {
		return nil, nil, translateDBErr(err)
	}
	it, err := s.store.getItem(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return t, &it.DictItem, nil
}

// ownedItem loads an item whose type the caller may modify.
func (s *Service) ownedItem(ctx context.Context, sc common.Scope, id uuid.UUID) (*itemRow, error) {
	it, err := s.store.getItem(ctx, id)
	if err != nil {
		return nil, err
	}
	if it == nil {
		return nil, httpx.NotFound("字典条目不存在")
	}
	if err := sc.CheckWrite(it.TypeTenantID, "字典条目不存在"); err != nil {
		return nil, err
	}
	return it, nil
}

func (s *Service) UpdateItem(ctx context.Context, sc common.Scope, id uuid.UUID, req ItemUpdateRequest) (*DictItem, *DictItem, error) {
	before, err := s.ownedItem(ctx, sc, id)
	if err != nil {
		return nil, nil, err
	}
	if req.Value != nil && strings.TrimSpace(*req.Value) == "" {
		return nil, nil, httpx.BadRequest("条目值不能为空")
	}
	if err := s.store.updateItem(ctx, id, req); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.store.getItem(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &before.DictItem, &after.DictItem, nil
}

func (s *Service) DeleteItem(ctx context.Context, sc common.Scope, id uuid.UUID) (*DictItem, error) {
	it, err := s.ownedItem(ctx, sc, id)
	if err != nil {
		return nil, err
	}
	if err := s.store.deleteItem(ctx, id); err != nil {
		return nil, err
	}
	return &it.DictItem, nil
}

// ByCode returns the enabled items of the tenant's type for code, falling
// back to the global type. Any authenticated user may call it.
func (s *Service) ByCode(ctx context.Context, tenantID uuid.UUID, code string) ([]DictItem, error) {
	t, err := s.store.resolveTypeByCode(ctx, tenantID, strings.TrimSpace(code))
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, httpx.NotFound("字典不存在")
	}
	return s.store.activeItems(ctx, t.ID)
}

// checkItemValues rejects blank or duplicate values within one request.
func checkItemValues(items []ItemCreateRequest) error {
	seen := map[string]struct{}{}
	for _, it := range items {
		v := strings.TrimSpace(it.Value)
		if v == "" {
			return httpx.BadRequest("条目值不能为空")
		}
		if _, dup := seen[v]; dup {
			return httpx.BadRequest("条目值重复：" + v)
		}
		seen[v] = struct{}{}
	}
	return nil
}

func translateDBErr(err error) error {
	switch cn := common.ConflictConstraint(err); {
	case cn == "":
		return err
	case cn == "uq_dict_types_code":
		return httpx.Conflict("字典代码已存在")
	case strings.HasPrefix(cn, "dict_items"):
		return httpx.Conflict("同一字典内条目值已存在")
	default:
		return httpx.Conflict("数据冲突：" + cn)
	}
}
