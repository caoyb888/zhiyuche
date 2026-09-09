// Package role manages roles, their permission sets, and exposes the
// permission registry as a tree.
//
// Scope: a tenant sees its own roles (roles.tenant_id = tenant); the platform
// tenant additionally sees platform-level roles (tenant_id IS NULL).
package role

import (
	"time"

	"github.com/google/uuid"
)

// Role is the API representation.
type Role struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TenantID    *uuid.UUID `json:"tenant_id" db:"tenant_id"` // nil = 平台级角色
	Code        string     `json:"code" db:"code"`
	Name        string     `json:"name" db:"name"`
	Description *string    `json:"description" db:"description"`
	IsSystem    bool       `json:"is_system" db:"is_system"`
	Permissions []string   `json:"permissions" db:"-"`
	UserCount   int64      `json:"user_count" db:"user_count"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Brief is the dropdown shape (GET /system/roles/options).
type Brief struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Code string    `json:"code" db:"code"`
	Name string    `json:"name" db:"name"`
}

// ListQuery mirrors the query string of GET /system/roles.
type ListQuery struct {
	Keyword string `form:"keyword"`
}

type CreateRequest struct {
	Code        string   `json:"code" binding:"required,max=32"`
	Name        string   `json:"name" binding:"required,min=1,max=64"`
	Description *string  `json:"description" binding:"omitempty,max=255"`
	Permissions []string `json:"permissions"`
}

// UpdateRequest: only name/description are editable (code never changes).
type UpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=64"`
	Description *string `json:"description" binding:"omitempty,max=255"`
}

type SetPermissionsRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
}

// PermissionNode is the registry tree shape (GET /system/permissions/tree).
type PermissionNode struct {
	Code     string            `json:"code"`
	Name     string            `json:"name"`
	Type     string            `json:"type"` // menu | action
	Path     string            `json:"path,omitempty"`
	Icon     string            `json:"icon,omitempty"`
	Children []*PermissionNode `json:"children,omitempty"`
}
