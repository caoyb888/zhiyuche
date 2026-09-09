// Package dept manages the department tree of a tenant.
//
// departments.path is the ancestor chain "/id1/id2/" including the node
// itself, so "is X inside the subtree of Y" is a plain prefix test and moving a
// subtree is one UPDATE that swaps the prefix.
package dept

import (
	"time"

	"github.com/google/uuid"
)

// Dept is the API representation.
type Dept struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	ParentID      *uuid.UUID `json:"parent_id" db:"parent_id"`
	Name          string     `json:"name" db:"name"`
	Code          *string    `json:"code" db:"code"`
	Path          string     `json:"path" db:"path"`
	Sort          int        `json:"sort" db:"sort"`
	LeaderUserID  *uuid.UUID `json:"leader_user_id" db:"leader_user_id"`
	LeaderName    *string    `json:"leader_name" db:"leader_name"`
	MonthlyBudget float64    `json:"monthly_budget" db:"monthly_budget"`
	Status        string     `json:"status" db:"status"`
	UserCount     int64      `json:"user_count" db:"user_count"` // 直属在职用户数
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// Node is a Dept with its children (GET /system/depts/tree).
type Node struct {
	Dept
	Children []*Node `json:"children"`
}

// ListQuery mirrors the query string of GET /system/depts.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=active disabled"`
}

type CreateRequest struct {
	ParentID      *uuid.UUID `json:"parent_id"`
	Name          string     `json:"name" binding:"required,min=1,max=64"`
	Code          *string    `json:"code" binding:"omitempty,max=64"`
	Sort          *int       `json:"sort"`
	LeaderUserID  *uuid.UUID `json:"leader_user_id"`
	MonthlyBudget *float64   `json:"monthly_budget" binding:"omitempty,gte=0"`
	Status        string     `json:"status" binding:"omitempty,oneof=active disabled"`
}

// UpdateRequest only touches fields that are present. parent_id moves the
// subtree; clear_parent makes it a root; clear_leader empties the leader.
type UpdateRequest struct {
	ParentID      *uuid.UUID `json:"parent_id"`
	ClearParent   bool       `json:"clear_parent"`
	Name          *string    `json:"name" binding:"omitempty,min=1,max=64"`
	Code          *string    `json:"code" binding:"omitempty,max=64"`
	Sort          *int       `json:"sort"`
	LeaderUserID  *uuid.UUID `json:"leader_user_id"`
	ClearLeader   bool       `json:"clear_leader"`
	MonthlyBudget *float64   `json:"monthly_budget" binding:"omitempty,gte=0"`
	Status        *string    `json:"status" binding:"omitempty,oneof=active disabled"`
}
