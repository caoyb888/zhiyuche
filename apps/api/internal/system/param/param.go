// Package param manages sys_params: global defaults with per-tenant overrides.
//
// Get is the read helper other modules use; the CRUD handlers are registered by
// Register (implemented in handler.go).
package param

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Well-known keys. Seeded globally by bootstrap.
const (
	KeyDefaultPassword     = "security.default_password"
	KeyLoginMaxFailures    = "security.login_max_failures"
	KeyApprovalTimeoutMins = "approval.overdue_alert_minutes"
	KeyLowSocLock          = "vehicle.low_soc_lock_percent"
)

// Defaults are inserted (tenant_id NULL) when missing.
var Defaults = []struct {
	Key, Value, Type, Description string
	Public                        bool
}{
	{KeyDefaultPassword, "Zy@123456", "string", "新建/重置用户时的缺省密码", false},
	{KeyLoginMaxFailures, "5", "int", "连续登录失败锁定阈值", false},
	{KeyApprovalTimeoutMins, "30", "int", "超过预计还车时间多少分钟触发提醒", true},
	{KeyLowSocLock, "10", "int", "SOC 低于该百分比禁止新行程", true},
}

// Get returns the tenant override if present, else the global value, else fallback.
func Get(ctx context.Context, db *pgxpool.Pool, tenantID uuid.UUID, key, fallback string) string {
	var v string
	err := db.QueryRow(ctx, `
		SELECT value FROM sys_params
		WHERE key = $1 AND (tenant_id = $2 OR tenant_id IS NULL)
		ORDER BY tenant_id NULLS LAST LIMIT 1`, key, tenantID).Scan(&v)
	if err != nil {
		return fallback
	}
	return v
}

// GetInt is Get with integer parsing.
func GetInt(ctx context.Context, db *pgxpool.Pool, tenantID uuid.UUID, key string, fallback int) int {
	v := Get(ctx, db, tenantID, key, "")
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return fallback
}
