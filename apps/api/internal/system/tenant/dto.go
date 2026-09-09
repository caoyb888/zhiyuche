// Package tenant manages tenants. Every endpoint needs a system:tenant:*
// permission, which only the platform tenant can hold — in practice the super
// admin. Tenants are never physically deleted; DELETE disables.
package tenant

import (
	"time"

	"github.com/google/uuid"
)

// Tenant is the API representation.
type Tenant struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Code         string     `json:"code" db:"code"`
	Name         string     `json:"name" db:"name"`
	LicenseNo    *string    `json:"license_no" db:"license_no"`
	ContactName  *string    `json:"contact_name" db:"contact_name"`
	ContactPhone *string    `json:"contact_phone" db:"contact_phone"`
	Status       string     `json:"status" db:"status"`
	IsPlatform   bool       `json:"is_platform" db:"is_platform"`
	ExpiresAt    *time.Time `json:"expires_at" db:"expires_at"`
	UserCount    int64      `json:"user_count" db:"user_count"`
	DeptCount    int64      `json:"dept_count" db:"dept_count"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// ListQuery mirrors the query string of GET /system/tenants.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=active disabled"`
}

type CreateRequest struct {
	Code          string     `json:"code" binding:"required,max=32"`
	Name          string     `json:"name" binding:"required,min=1,max=128"`
	LicenseNo     *string    `json:"license_no" binding:"omitempty,max=64"`
	ContactName   *string    `json:"contact_name" binding:"omitempty,max=64"`
	ContactPhone  *string    `json:"contact_phone" binding:"omitempty,max=32"`
	ExpiresAt     *time.Time `json:"expires_at"`
	AdminUsername string     `json:"admin_username" binding:"required,min=2,max=64"`
	AdminPassword string     `json:"admin_password" binding:"omitempty,max=72"` // empty → 系统缺省密码
	AdminName     string     `json:"admin_name" binding:"omitempty,max=64"`     // empty → 管理员
}

// UpdateRequest only touches fields that are present; clear_expires empties expires_at.
type UpdateRequest struct {
	Name         *string    `json:"name" binding:"omitempty,min=1,max=128"`
	LicenseNo    *string    `json:"license_no" binding:"omitempty,max=64"`
	ContactName  *string    `json:"contact_name" binding:"omitempty,max=64"`
	ContactPhone *string    `json:"contact_phone" binding:"omitempty,max=32"`
	Status       *string    `json:"status" binding:"omitempty,oneof=active disabled"`
	ExpiresAt    *time.Time `json:"expires_at"`
	ClearExpires bool       `json:"clear_expires"`
}
