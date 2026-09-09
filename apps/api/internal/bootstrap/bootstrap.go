// Package bootstrap makes a freshly migrated database usable: syncs the
// permission registry, creates the platform tenant + super admin, seeds global
// parameters, and (optionally) a demo tenant. Every step is idempotent and runs
// on each start.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/config"
	"github.com/caoyb888/zhiyuche/apps/api/internal/perm"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/param"
)

const (
	PlatformTenantCode = "platform"
	DemoTenantCode     = "chenhua"
)

func Run(ctx context.Context, db *pgxpool.Pool, cfg *config.Config, log zerolog.Logger) error {
	newCodes, err := SyncPermissions(ctx, db)
	if err != nil {
		return fmt.Errorf("sync permissions: %w", err)
	}
	if len(newCodes) > 0 {
		log.Info().Strs("codes", newCodes).Msg("new permission codes registered, granting to tenant admins")
		if err := propagateNewPerms(ctx, db, newCodes); err != nil {
			return fmt.Errorf("propagate permissions: %w", err)
		}
	}
	if err := seedParams(ctx, db); err != nil {
		return fmt.Errorf("seed params: %w", err)
	}
	platformID, err := ensureTenant(ctx, db, PlatformTenantCode, "智御平台", true)
	if err != nil {
		return fmt.Errorf("platform tenant: %w", err)
	}
	if err := ensureSuperAdmin(ctx, db, platformID, cfg, log); err != nil {
		return fmt.Errorf("super admin: %w", err)
	}
	if err := EnsureTenantDefaults(ctx, db, platformID); err != nil {
		return err
	}
	if cfg.Bootstrap.DemoTenant {
		demoID, err := ensureTenant(ctx, db, DemoTenantCode, "山东宸华", false)
		if err != nil {
			return fmt.Errorf("demo tenant: %w", err)
		}
		if err := EnsureTenantDefaults(ctx, db, demoID); err != nil {
			return err
		}
		if err := EnsureTenantAdmin(ctx, db, demoID, "chenhua_admin", "宸华管理员", cfg.Bootstrap.AdminPassword, log); err != nil {
			return fmt.Errorf("demo admin: %w", err)
		}
	}
	return nil
}

// SyncPermissions upserts the registry and removes rows no longer registered.
// It returns the action codes that did not exist before this run.
func SyncPermissions(ctx context.Context, db *pgxpool.Pool) ([]string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	codes := make([]string, 0, len(perm.Defs))
	var newActions []string
	// parents first: Defs is ordered so parents precede children
	for _, d := range perm.Defs {
		codes = append(codes, d.Code)
		var parent *string
		if d.Parent != "" {
			p := d.Parent
			parent = &p
		}
		var inserted bool
		err := tx.QueryRow(ctx, `
			INSERT INTO permissions (code, name, type, parent_code, path, icon, sort, updated_at)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7, now())
			ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, parent_code = EXCLUDED.parent_code,
			  path = EXCLUDED.path, icon = EXCLUDED.icon, sort = EXCLUDED.sort, updated_at = now()
			RETURNING (xmax = 0) AS inserted`,
			d.Code, d.Name, string(d.Type), parent, d.Path, d.Icon, d.Sort).Scan(&inserted)
		if err != nil {
			return nil, fmt.Errorf("upsert %s: %w", d.Code, err)
		}
		if inserted && d.Type == perm.Action {
			newActions = append(newActions, d.Code)
		}
	}
	// delete children before parents (FK) — rows not in registry
	if _, err := tx.Exec(ctx, `DELETE FROM permissions WHERE code <> ALL($1) AND type = 'action'`, codes); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM permissions WHERE code <> ALL($1)`, codes); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return newActions, nil
}

// propagateNewPerms grants freshly registered action codes to every existing
// tenant_admin role (platform-only codes only to the platform tenant) and to
// super_admin. Codes an administrator deliberately removed are never re-added,
// because only codes that were absent from the table are passed in.
func propagateNewPerms(ctx context.Context, db *pgxpool.Pool, codes []string) error {
	var platformOnly []string
	for c := range perm.PlatformOnly {
		platformOnly = append(platformOnly, c)
	}
	_, err := db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_code)
		SELECT r.id, c
		FROM roles r
		JOIN tenants t ON t.id = r.tenant_id
		CROSS JOIN unnest($1::text[]) AS c
		WHERE r.code = 'tenant_admin' AND r.deleted_at IS NULL
		  AND (t.is_platform OR c <> ALL($2::text[]))
		ON CONFLICT DO NOTHING`, codes, platformOnly)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_code)
		SELECT r.id, c FROM roles r CROSS JOIN unnest($1::text[]) AS c
		WHERE r.code = 'super_admin' AND r.tenant_id IS NULL AND r.deleted_at IS NULL
		ON CONFLICT DO NOTHING`, codes)
	return err
}

func seedParams(ctx context.Context, db *pgxpool.Pool) error {
	for _, p := range param.Defaults {
		_, err := db.Exec(ctx, `
			INSERT INTO sys_params (tenant_id, key, value, value_type, description, is_public)
			SELECT NULL, $1, $2, $3, $4, $5
			WHERE NOT EXISTS (SELECT 1 FROM sys_params WHERE tenant_id IS NULL AND key = $1)`,
			p.Key, p.Value, p.Type, p.Description, p.Public)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureTenant(ctx context.Context, db *pgxpool.Pool, code, name string, platform bool) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `SELECT id FROM tenants WHERE code = $1`, code).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	err = db.QueryRow(ctx, `INSERT INTO tenants (code, name, is_platform) VALUES ($1,$2,$3) RETURNING id`, code, name, platform).Scan(&id)
	return id, err
}

// EnsureTenantDefaults creates the built-in roles for a tenant and keeps
// tenant_admin's permission set complete. Safe to call repeatedly; also used by
// the tenant module when a tenant is created.
func EnsureTenantDefaults(ctx context.Context, db *pgxpool.Pool, tenantID uuid.UUID) error {
	var isPlatform bool
	if err := db.QueryRow(ctx, `SELECT is_platform FROM tenants WHERE id = $1`, tenantID).Scan(&isPlatform); err != nil {
		return err
	}
	for _, r := range perm.DefaultRoles {
		roleID, created, err := ensureRole(ctx, db, &tenantID, r)
		if err != nil {
			return fmt.Errorf("role %s: %w", r.Code, err)
		}
		perms := r.Perms
		if r.Perms == nil {
			perms = perm.ActionCodes(isPlatform)
		}
		// 只在角色刚创建时写入默认权限集；已有角色的权限由管理员维护，
		// 注册表新增的权限码由 propagateNewPerms 定向补给 tenant_admin
		if created {
			if err := setRolePerms(ctx, db, roleID, perms); err != nil {
				return err
			}
		}
	}
	if isPlatform {
		roleID, created, err := ensureRole(ctx, db, nil, perm.SuperAdminRole)
		if err != nil {
			return err
		}
		if created {
			if err := setRolePerms(ctx, db, roleID, perm.ActionCodes(true)); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureRole(ctx context.Context, db *pgxpool.Pool, tenantID *uuid.UUID, r perm.DefaultRole) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		SELECT id FROM roles WHERE code = $1 AND deleted_at IS NULL
		  AND COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)`,
		r.Code, tenantID).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, err
	}
	err = db.QueryRow(ctx, `INSERT INTO roles (tenant_id, code, name, description, is_system) VALUES ($1,$2,$3,$4,true) RETURNING id`,
		tenantID, r.Code, r.Name, r.Description).Scan(&id)
	return id, true, err
}

func setRolePerms(ctx context.Context, db *pgxpool.Pool, roleID uuid.UUID, codes []string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	for _, code := range codes {
		if _, err := tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_code) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, code); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ensureSuperAdmin(ctx context.Context, db *pgxpool.Pool, platformID uuid.UUID, cfg *config.Config, log zerolog.Logger) error {
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_super AND deleted_at IS NULL`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := auth.HashPassword(cfg.Bootstrap.AdminPassword)
	if err != nil {
		return err
	}
	var uid uuid.UUID
	err = db.QueryRow(ctx, `
		INSERT INTO users (tenant_id, username, password_hash, name, status, is_super, password_changed_at)
		VALUES ($1,$2,$3,'超级管理员','active',true, now()) RETURNING id`,
		platformID, cfg.Bootstrap.AdminUsername, hash).Scan(&uid)
	if err != nil {
		return err
	}
	roleID, _, err := ensureRole(ctx, db, nil, perm.SuperAdminRole)
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, uid, roleID); err != nil {
		return err
	}
	log.Warn().Str("username", cfg.Bootstrap.AdminUsername).Msg("super admin created with bootstrap password — change it after first login")
	return nil
}

// EnsureTenantAdmin creates the tenant administrator account (bound to the
// tenant's tenant_admin role) when no user of that name exists yet. Idempotent;
// also used by the tenant module when a tenant is created.
func EnsureTenantAdmin(ctx context.Context, db *pgxpool.Pool, tenantID uuid.UUID, username, name, password string, log zerolog.Logger) error {
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND username = $2 AND deleted_at IS NULL`, tenantID, username).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	var uid uuid.UUID
	err = db.QueryRow(ctx, `
		INSERT INTO users (tenant_id, username, password_hash, name, status, password_changed_at)
		VALUES ($1,$2,$3,$4,'active', now()) RETURNING id`, tenantID, username, hash, name).Scan(&uid)
	if err != nil {
		return err
	}
	var roleID uuid.UUID
	if err := db.QueryRow(ctx, `SELECT id FROM roles WHERE tenant_id = $1 AND code = 'tenant_admin' AND deleted_at IS NULL`, tenantID).Scan(&roleID); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, uid, roleID); err != nil {
		return err
	}
	log.Info().Str("username", username).Str("tenant_id", tenantID.String()).Msg("tenant admin created")
	return nil
}
