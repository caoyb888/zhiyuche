// Package template manages notification templates: global defaults
// (tenant_id NULL) overridable per tenant on (code, channel).
package template

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Template struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	TenantID  *uuid.UUID `json:"tenant_id" db:"tenant_id"`
	IsGlobal  bool       `json:"is_global" db:"is_global"`
	Code      string     `json:"code" db:"code"`
	Channel   string     `json:"channel" db:"channel"`
	Title     string     `json:"title" db:"title"`
	Content   string     `json:"content" db:"content"`
	Enabled   bool       `json:"enabled" db:"enabled"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// ListQuery mirrors the query string of GET /system/notification-templates.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Channel string `form:"channel" binding:"omitempty,oneof=inapp wechat sms"`
}

type CreateRequest struct {
	Code    string `json:"code" binding:"required,max=64"`
	Channel string `json:"channel" binding:"required,oneof=inapp wechat sms"`
	Title   string `json:"title" binding:"required,min=1,max=128"`
	Content string `json:"content" binding:"required,min=1,max=4000"`
	Enabled *bool  `json:"enabled"` // nil → true
}

type UpdateRequest struct {
	Title   *string `json:"title" binding:"omitempty,min=1,max=128"`
	Content *string `json:"content" binding:"omitempty,min=1,max=4000"`
	Enabled *bool   `json:"enabled"`
}

// codePattern is the contract's TemplateCreate.code pattern (event codes such as approval.submitted).
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_.]{1,63}$`)

// ValidCode reports whether code satisfies ^[a-z][a-z0-9_.]{1,63}$.
func ValidCode(code string) bool { return codePattern.MatchString(code) }
