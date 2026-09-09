// Package auditlog exposes read-only queries over audit_logs.
package auditlog

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// AuditLog mirrors one audit_logs row. Before/After are only loaded by the
// detail endpoint; the list leaves them null.
type AuditLog struct {
	ID         int64           `json:"id" db:"id"`
	TenantID   *uuid.UUID      `json:"tenant_id" db:"tenant_id"`
	UserID     *uuid.UUID      `json:"user_id" db:"user_id"`
	Username   *string         `json:"username" db:"username"`
	Action     string          `json:"action" db:"action"`
	Module     string          `json:"module" db:"module"`
	TargetType *string         `json:"target_type" db:"target_type"`
	TargetID   *string         `json:"target_id" db:"target_id"`
	Summary    *string         `json:"summary" db:"summary"`
	Before     json.RawMessage `json:"before" db:"before"`
	After      json.RawMessage `json:"after" db:"after"`
	Method     string          `json:"method" db:"method"`
	Path       string          `json:"path" db:"path"`
	Status     int             `json:"status" db:"status"`
	IP         *string         `json:"ip" db:"ip"`
	UserAgent  *string         `json:"user_agent" db:"user_agent"`
	RequestID  *string         `json:"request_id" db:"request_id"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// ListQuery mirrors the query string of GET /system/audit-logs.
type ListQuery struct {
	UserID   string `form:"user_id"`
	Username string `form:"username"`
	Module   string `form:"module"`
	Action   string `form:"action"`
	TargetID string `form:"target_id"`
	Keyword  string `form:"keyword"`
	From     string `form:"from"`
	To       string `form:"to"`
}

// filter is the parsed, validated form of ListQuery.
type filter struct {
	UserID   *uuid.UUID
	Username string
	Module   string
	Action   string
	TargetID string
	Keyword  string
	From, To *time.Time
}

// parseFilter validates user_id / from / to. Times accept RFC3339 or a bare
// date (YYYY-MM-DD, interpreted in the server's local zone).
func parseFilter(q ListQuery) (filter, error) {
	f := filter{
		Username: strings.TrimSpace(q.Username),
		Module:   strings.TrimSpace(q.Module),
		Action:   strings.TrimSpace(q.Action),
		TargetID: strings.TrimSpace(q.TargetID),
		Keyword:  strings.TrimSpace(q.Keyword),
	}
	if s := strings.TrimSpace(q.UserID); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return f, httpx.BadRequest("invalid user_id: must be a uuid")
		}
		f.UserID = &id
	}
	var err error
	if f.From, err = parseTime(q.From, "from"); err != nil {
		return f, err
	}
	if f.To, err = parseTime(q.To, "to"); err != nil {
		return f, err
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		return f, httpx.BadRequest("to must not be before from")
	}
	return f, nil
}

func parseTime(s, name string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		var t time.Time
		var err error
		if layout == time.RFC3339 || layout == time.RFC3339Nano {
			t, err = time.Parse(layout, s)
		} else {
			t, err = time.ParseInLocation(layout, s, time.Local)
		}
		if err == nil {
			return &t, nil
		}
	}
	return nil, httpx.BadRequest("invalid " + name + ": expected RFC3339 date-time")
}
