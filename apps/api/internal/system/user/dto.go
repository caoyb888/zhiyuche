// Package user implements user (employee account) management. It is the
// reference implementation for the store / service / handler layout every
// system module follows.
package user

import (
	"time"

	"github.com/google/uuid"
)

// User is the API representation.
type User struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	TenantID    uuid.UUID   `json:"tenant_id" db:"tenant_id"`
	DeptID      *uuid.UUID  `json:"dept_id" db:"dept_id"`
	DeptName    *string     `json:"dept_name" db:"dept_name"`
	Username    string      `json:"username" db:"username"`
	Name        string      `json:"name" db:"name"`
	Phone       *string     `json:"phone" db:"phone"`
	Email       *string     `json:"email" db:"email"`
	EmployeeNo  *string     `json:"employee_no" db:"employee_no"`
	AvatarURL   *string     `json:"avatar_url" db:"avatar_url"`
	Status      string      `json:"status" db:"status"`
	IsSuper     bool        `json:"is_super" db:"is_super"`
	LastLoginAt *time.Time  `json:"last_login_at" db:"last_login_at"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
	Roles       []RoleBrief `json:"roles" db:"-"`
}

type RoleBrief struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Code string    `json:"code" db:"code"`
	Name string    `json:"name" db:"name"`
}

// ListQuery mirrors the query string of GET /system/users.
type ListQuery struct {
	Keyword string `form:"keyword"`
	DeptID  string `form:"dept_id"`
	Status  string `form:"status" binding:"omitempty,oneof=active disabled locked"`
	RoleID  string `form:"role_id"`
}

// OptionsQuery / UserOption back GET /system/users/options (any authenticated user).
type OptionsQuery struct {
	Keyword string `form:"keyword" binding:"max=64"`
	DeptID  string `form:"dept_id"`
	Limit   int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

type UserOption struct {
	ID       uuid.UUID `json:"id" db:"id"`
	Name     string    `json:"name" db:"name"`
	Username string    `json:"username" db:"username"`
	DeptName *string   `json:"dept_name" db:"dept_name"`
	Phone    *string   `json:"phone" db:"phone"`
}

type CreateRequest struct {
	Username   string      `json:"username" binding:"required,min=2,max=64"`
	Name       string      `json:"name" binding:"required,min=1,max=64"`
	Password   string      `json:"password" binding:"omitempty,max=72"` // empty → 系统缺省密码
	Phone      *string     `json:"phone" binding:"omitempty,max=32"`
	Email      *string     `json:"email" binding:"omitempty,email,max=128"`
	EmployeeNo *string     `json:"employee_no" binding:"omitempty,max=64"`
	DeptID     *uuid.UUID  `json:"dept_id"`
	RoleIDs    []uuid.UUID `json:"role_ids"`
	Status     string      `json:"status" binding:"omitempty,oneof=active disabled"`
}

type UpdateRequest struct {
	Name       *string    `json:"name" binding:"omitempty,min=1,max=64"`
	Phone      *string    `json:"phone" binding:"omitempty,max=32"`
	Email      *string    `json:"email" binding:"omitempty,email,max=128"`
	EmployeeNo *string    `json:"employee_no" binding:"omitempty,max=64"`
	AvatarURL  *string    `json:"avatar_url" binding:"omitempty,max=512"`
	DeptID     *uuid.UUID `json:"dept_id"`
	ClearDept  bool       `json:"clear_dept"` // true → dept_id 置空
	Status     *string    `json:"status" binding:"omitempty,oneof=active disabled locked"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"omitempty,max=72"` // empty → 系统缺省密码
}

type ResetPasswordResponse struct {
	Password string `json:"password"` // 明文返回一次，供管理员告知用户
}

type AssignRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids" binding:"required"`
}
