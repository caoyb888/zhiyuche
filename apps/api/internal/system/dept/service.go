package dept

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

type Service struct {
	app   *app.App
	store *store
}

func NewService(a *app.App) *Service {
	return &Service{app: a, store: &store{db: a.DB}}
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Dept, error) {
	rows, err := s.store.list(ctx, tenantID, q)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Dept{}
	}
	return rows, nil
}

// Tree returns the whole department tree of the tenant.
func (s *Service) Tree(ctx context.Context, tenantID uuid.UUID) ([]*Node, error) {
	rows, err := s.store.list(ctx, tenantID, ListQuery{})
	if err != nil {
		return nil, err
	}
	return BuildTree(rows), nil
}

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Dept, error) {
	d, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, httpx.NotFound("部门不存在")
	}
	return d, nil
}

func (s *Service) checkLeader(ctx context.Context, tenantID, userID uuid.UUID) error {
	ok, err := s.store.userExists(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.BadRequest("负责人不存在或不属于本租户")
	}
	return nil
}

func (s *Service) checkName(ctx context.Context, tenantID uuid.UUID, parentID *uuid.UUID, name string, exclude uuid.UUID) error {
	exists, err := s.store.nameExists(ctx, tenantID, parentID, name, exclude)
	if err != nil {
		return err
	}
	if exists {
		return httpx.Conflict("同一上级下已存在同名部门")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest) (*Dept, error) {
	parentPath := ""
	if req.ParentID != nil {
		p, err := s.store.get(ctx, tenantID, *req.ParentID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, httpx.BadRequest("上级部门不存在")
		}
		parentPath = p.Path
	}
	if err := s.checkName(ctx, tenantID, req.ParentID, req.Name, uuid.Nil); err != nil {
		return nil, err
	}
	if req.LeaderUserID != nil {
		if err := s.checkLeader(ctx, tenantID, *req.LeaderUserID); err != nil {
			return nil, err
		}
	}
	id := uuid.New()
	if err := s.store.create(ctx, tenantID, id, ChildPath(parentPath, id), req); err != nil {
		return nil, translateDBErr(err)
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) (*Dept, *Dept, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}

	// ---- 移动（改上级）----
	var mv *move
	targetParent := before.ParentID
	if req.ClearParent || req.ParentID != nil {
		var newParent *Dept
		if !req.ClearParent {
			if *req.ParentID == id {
				return nil, nil, httpx.BadRequest("不能将部门移动到自身下")
			}
			newParent, err = s.store.get(ctx, tenantID, *req.ParentID)
			if err != nil {
				return nil, nil, err
			}
			if newParent == nil {
				return nil, nil, httpx.BadRequest("上级部门不存在")
			}
			if InSubtree(newParent.Path, before.Path) {
				return nil, nil, httpx.BadRequest("不能将部门移动到自身的子部门下")
			}
		}
		changed := (newParent == nil) != (before.ParentID == nil) ||
			(newParent != nil && before.ParentID != nil && newParent.ID != *before.ParentID)
		if changed {
			parentPath := ""
			var pid *uuid.UUID
			if newParent != nil {
				parentPath = newParent.Path
				pid = &newParent.ID
			}
			mv = &move{parentID: pid, oldPath: before.Path, newPath: ChildPath(parentPath, id)}
			targetParent = pid
		}
	}

	// ---- 同级重名 ----
	name := before.Name
	if req.Name != nil {
		name = *req.Name
	}
	if req.Name != nil || mv != nil {
		if err := s.checkName(ctx, tenantID, targetParent, name, id); err != nil {
			return nil, nil, err
		}
	}
	if req.LeaderUserID != nil && !req.ClearLeader {
		if err := s.checkLeader(ctx, tenantID, *req.LeaderUserID); err != nil {
			return nil, nil, err
		}
	}
	if err := s.store.update(ctx, tenantID, id, req, mv); err != nil {
		return nil, nil, translateDBErr(err)
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) (*Dept, error) {
	d, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if n, err := s.store.childCount(ctx, id); err != nil {
		return nil, err
	} else if n > 0 {
		return nil, httpx.Conflict("存在子部门，无法删除")
	}
	if n, err := s.store.userCount(ctx, id); err != nil {
		return nil, err
	} else if n > 0 {
		return nil, httpx.Conflict("部门下仍有在职用户，无法删除")
	}
	if err := s.store.softDelete(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return d, nil
}

// translateDBErr maps constraint violations to readable client errors.
func translateDBErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return httpx.Conflict("数据冲突：" + pgErr.ConstraintName)
		case "23503":
			return httpx.BadRequest("关联数据不存在：" + pgErr.ConstraintName)
		}
	}
	return err
}
