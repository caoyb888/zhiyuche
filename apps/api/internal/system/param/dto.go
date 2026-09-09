package param

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Param is the effective value of one key for a tenant: the tenant override
// when present (source=tenant), else the global default (source=global).
type Param struct {
	Key         string    `json:"key" db:"key"`
	Value       string    `json:"value" db:"value"`
	ValueType   string    `json:"value_type" db:"value_type"`
	Description *string   `json:"description" db:"description"`
	IsPublic    bool      `json:"is_public" db:"is_public"`
	Source      string    `json:"source" db:"source"`
	GlobalValue *string   `json:"global_value" db:"global_value"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ListQuery mirrors the query string of GET /system/params.
type ListQuery struct {
	Keyword string `form:"keyword"`
}

// UpdateRequest is the body of PUT /system/params/{key}. Value is a pointer so
// an explicit empty string is accepted for string-typed params.
type UpdateRequest struct {
	Value       *string `json:"value" binding:"required"`
	Description *string `json:"description" binding:"omitempty,max=255"`
}

// ValidateValue checks that value can be parsed as valueType
// (string | int | float | bool | json).
func ValidateValue(valueType, value string) error {
	switch valueType {
	case "string", "":
		return nil
	case "int":
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("值必须是整数")
		}
	case "float":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("值必须是数字")
		}
	case "bool":
		switch value {
		case "true", "false":
		default:
			return fmt.Errorf("值必须是 true 或 false")
		}
	case "json":
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("值必须是合法 JSON")
		}
	default:
		return fmt.Errorf("未知的参数类型 %s", valueType)
	}
	return nil
}
