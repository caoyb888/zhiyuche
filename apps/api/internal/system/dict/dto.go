// Package dict manages dictionary types and items. Types are either global
// (tenant_id NULL, visible to every tenant) or tenant-owned; a tenant type
// with the same code overrides the global one for that tenant.
package dict

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// DictType is the API representation. Items is only filled by the detail
// endpoint; the list endpoint returns an empty array.
type DictType struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TenantID    *uuid.UUID `json:"tenant_id" db:"tenant_id"`
	IsGlobal    bool       `json:"is_global" db:"is_global"`
	Code        string     `json:"code" db:"code"`
	Name        string     `json:"name" db:"name"`
	Description *string    `json:"description" db:"description"`
	IsSystem    bool       `json:"is_system" db:"is_system"`
	Items       []DictItem `json:"items" db:"-"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

type DictItem struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	DictTypeID uuid.UUID       `json:"dict_type_id" db:"dict_type_id"`
	Label      string          `json:"label" db:"label"`
	Value      string          `json:"value" db:"value"`
	Sort       int             `json:"sort" db:"sort"`
	Color      *string         `json:"color" db:"color"`
	Status     string          `json:"status" db:"status"`
	Extra      json.RawMessage `json:"extra" db:"extra"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// ListQuery mirrors the query string of GET /system/dict-types.
type ListQuery struct {
	Keyword string `form:"keyword"`
}

type CreateRequest struct {
	Code        string              `json:"code" binding:"required,max=64"`
	Name        string              `json:"name" binding:"required,min=1,max=64"`
	Description *string             `json:"description" binding:"omitempty,max=255"`
	Items       []ItemCreateRequest `json:"items" binding:"omitempty,dive"`
}

type UpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=64"`
	Description *string `json:"description" binding:"omitempty,max=255"`
}

type ItemCreateRequest struct {
	Label  string         `json:"label" binding:"required,min=1,max=64"`
	Value  string         `json:"value" binding:"required,min=1,max=64"`
	Sort   int            `json:"sort"`
	Color  *string        `json:"color" binding:"omitempty,max=32"`
	Status string         `json:"status" binding:"omitempty,oneof=active disabled"`
	Extra  map[string]any `json:"extra"`
}

type ItemUpdateRequest struct {
	Label  *string        `json:"label" binding:"omitempty,min=1,max=64"`
	Value  *string        `json:"value" binding:"omitempty,min=1,max=64"`
	Sort   *int           `json:"sort"`
	Color  *string        `json:"color" binding:"omitempty,max=32"`
	Status *string        `json:"status" binding:"omitempty,oneof=active disabled"`
	Extra  map[string]any `json:"extra"`
}

// codePattern is the contract's DictTypeCreate.code pattern.
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// ValidCode reports whether code satisfies ^[a-z][a-z0-9_]{1,63}$.
func ValidCode(code string) bool { return codePattern.MatchString(code) }

// extraJSON encodes the request's extra map; nil → "{}".
func extraJSON(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}
