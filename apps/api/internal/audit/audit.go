// Package audit records every state-changing request into audit_logs.
//
// The middleware writes one row per non-GET request after the handler finishes.
// Handlers enrich the row through Record(c, Entry{...}); without it the row
// still carries method/path/status/actor and a module derived from the path.
package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Actor is what the middleware needs to know about the caller.
type Actor struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Username string
}

// ActorResolver extracts the caller from the request (injected by the server so
// this package does not depend on the auth package).
type ActorResolver func(c *gin.Context) (Actor, bool)

// Entry is what a handler knows about the change it made.
type Entry struct {
	Module     string // e.g. "system.user"
	Action     string // create | update | delete | login | ...
	TargetType string
	TargetID   string
	Summary    string
	Before     any
	After      any
	// Actor overrides the principal (used by login, where no principal exists yet).
	ActorID   uuid.UUID
	ActorName string
	TenantID  uuid.UUID
}

const ctxKey = "audit.entry"

// Record attaches audit details to the current request.
func Record(c *gin.Context, e Entry) { c.Set(ctxKey, e) }

// Skip marks the request as not to be audited (e.g. token refresh).
func Skip(c *gin.Context) { c.Set(ctxKey+".skip", true) }

type Writer struct {
	db      *pgxpool.Pool
	log     zerolog.Logger
	resolve ActorResolver
}

func NewWriter(db *pgxpool.Pool, log zerolog.Logger, resolve ActorResolver) *Writer {
	return &Writer{db: db, log: log, resolve: resolve}
}

// Middleware audits every non-GET request.
func (w *Writer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions || c.Request.Method == http.MethodHead {
			return
		}
		if skip, _ := c.Get(ctxKey + ".skip"); skip == true {
			return
		}
		var e Entry
		if v, ok := c.Get(ctxKey); ok {
			e, _ = v.(Entry)
		}
		row := w.buildRow(c, e)
		go w.insert(row)
	}
}

type row struct {
	TenantID   *uuid.UUID
	UserID     *uuid.UUID
	Username   *string
	Action     string
	Module     string
	TargetType *string
	TargetID   *string
	Summary    *string
	Before     []byte
	After      []byte
	Method     string
	Path       string
	Status     int
	IP         string
	UserAgent  string
	RequestID  string
}

func (w *Writer) buildRow(c *gin.Context, e Entry) row {
	r := row{
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Status:    c.Writer.Status(),
		IP:        c.ClientIP(),
		UserAgent: truncate(c.Request.UserAgent(), 512),
		RequestID: httpx.RequestID(c),
		Module:    e.Module,
		Action:    e.Action,
	}
	if w.resolve != nil {
		if a, ok := w.resolve(c); ok {
			uid, tid, name := a.UserID, a.TenantID, a.Username
			r.UserID, r.TenantID, r.Username = &uid, &tid, &name
		}
	}
	if e.ActorID != uuid.Nil {
		uid := e.ActorID
		r.UserID = &uid
	}
	if e.ActorName != "" {
		n := e.ActorName
		r.Username = &n
	}
	if e.TenantID != uuid.Nil {
		t := e.TenantID
		r.TenantID = &t
	}
	if r.Module == "" {
		r.Module = moduleFromPath(c.FullPath())
	}
	if r.Action == "" {
		r.Action = actionFromMethod(c.Request.Method)
	}
	if e.TargetType != "" {
		r.TargetType = &e.TargetType
	}
	if e.TargetID != "" {
		r.TargetID = &e.TargetID
	}
	if e.Summary != "" {
		s := truncate(e.Summary, 500)
		r.Summary = &s
	}
	if e.Before != nil {
		r.Before, _ = json.Marshal(e.Before)
	}
	if e.After != nil {
		r.After, _ = json.Marshal(e.After)
	}
	return r
}

func (w *Writer) insert(r row) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := w.db.Exec(ctx, `
		INSERT INTO audit_logs
		  (tenant_id, user_id, username, action, module, target_type, target_id, summary, before, after,
		   method, path, status, ip, user_agent, request_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		r.TenantID, r.UserID, r.Username, r.Action, r.Module, r.TargetType, r.TargetID, r.Summary,
		nullableJSON(r.Before), nullableJSON(r.After), r.Method, r.Path, r.Status, r.IP, r.UserAgent, r.RequestID)
	if err != nil {
		w.log.Error().Err(err).Str("request_id", r.RequestID).Msg("audit insert failed")
	}
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// resourceAliases maps URL resource segments to the module names handlers use,
// so failed requests (which never reach audit.Record) land under the same module.
var resourceAliases = map[string]string{
	"users": "user", "depts": "dept", "roles": "role", "permissions": "permission", "tenants": "tenant",
	"dict-types": "dict", "dict-items": "dict", "dicts": "dict", "params": "param",
	"audit-logs": "audit", "notification-templates": "template",
}

// moduleFromPath turns "/api/v1/system/users/:id/roles" into "system.user".
func moduleFromPath(full string) string {
	full = strings.TrimPrefix(full, "/api/v1/")
	var parts []string
	for _, seg := range strings.Split(full, "/") {
		if seg == "" || strings.HasPrefix(seg, ":") {
			continue
		}
		if alias, ok := resourceAliases[seg]; ok {
			seg = alias
		}
		parts = append(parts, seg)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		return "unknown"
	}
	if parts[0] == "auth" {
		return "auth"
	}
	return strings.Join(parts, ".")
}

func actionFromMethod(m string) string {
	switch m {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	}
	return strings.ToLower(m)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
