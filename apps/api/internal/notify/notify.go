// Package notify stores in-app notifications and pushes them over WebSocket.
// Other modules call Service.Send; users read them via /notifications.
package notify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

// Notification types.
const (
	TypeApprovalPending  = "approval.pending"  // 待你审批
	TypeApprovalApproved = "approval.approved" // 你的申请已通过
	TypeApprovalRejected = "approval.rejected"
	TypeApprovalCancelled = "approval.cancelled"
	TypeTripStarted      = "trip.started"
	TypeTripEnded        = "trip.ended"
	TypeTripEvent        = "trip.event" // 偏离/超速/低电 等
	TypeSystem           = "system"
)

type Notification struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	TenantID  uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Type      string     `json:"type" db:"type"`
	Title     string     `json:"title" db:"title"`
	Content   string     `json:"content" db:"content"`
	RefType   *string    `json:"ref_type" db:"ref_type"`
	RefID     *string    `json:"ref_id" db:"ref_id"`
	ReadAt    *time.Time `json:"read_at" db:"read_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// Message is what a module sends.
type Message struct {
	Type    string
	Title   string
	Content string
	RefType string
	RefID   string
}

type Service struct{ app *app.App }

func NewService(a *app.App) *Service { return &Service{app: a} }

// Send stores one notification per user and pushes it to their live connections.
func (s *Service) Send(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID, m Message) error {
	seen := map[uuid.UUID]bool{}
	for _, uid := range userIDs {
		if uid == uuid.Nil || seen[uid] {
			continue
		}
		seen[uid] = true
		var n Notification
		err := pgxscan.Get(ctx, s.app.DB, &n, `
			INSERT INTO notifications (tenant_id, user_id, type, title, content, ref_type, ref_id)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''))
			RETURNING *`, tenantID, uid, m.Type, m.Title, m.Content, m.RefType, m.RefID)
		if err != nil {
			return fmt.Errorf("insert notification: %w", err)
		}
		s.app.Hub.PublishUser(uid, ws.Event{Type: ws.EvNotification, Data: n})
	}
	return nil
}

// ---- HTTP ----

// Register mounts /notifications (own notifications only; no permission code needed).
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/notifications")
	r.GET("", h.list)
	r.GET("/unread-count", h.unreadCount)
	r.PUT("/read", h.markRead)
	r.PUT("/read-all", h.markAllRead)
}

type handler struct{ svc *Service }

type listQuery struct {
	Unread bool   `form:"unread"`
	Type   string `form:"type"`
}

func (h *handler) list(c *gin.Context) {
	var q listQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	pg := pagination.Parse(c)
	uid := auth.Current(c).UserID
	where := []string{"user_id = $1"}
	args := []any{uid}
	if q.Unread {
		where = append(where, "read_at IS NULL")
	}
	if q.Type != "" {
		args = append(args, q.Type)
		where = append(where, fmt.Sprintf("type = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")
	ctx := c.Request.Context()
	var total int64
	if err := h.svc.app.DB.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE `+cond, args...).Scan(&total); err != nil {
		httpx.Fail(c, err)
		return
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Notification
	if err := pgxscan.Select(ctx, h.svc.app.DB, &rows,
		fmt.Sprintf(`SELECT * FROM notifications WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, cond, len(args)-1, len(args)), args...); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, pagination.NewPage(rows, total, pg))
}

func (h *handler) unreadCount(c *gin.Context) {
	var n int64
	if err := h.svc.app.DB.QueryRow(c.Request.Context(), `SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, auth.Current(c).UserID).Scan(&n); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"unread": n})
}

type readRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required,min=1"`
}

func (h *handler) markRead(c *gin.Context) {
	var req readRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	tag, err := h.svc.app.DB.Exec(c.Request.Context(),
		`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND id = ANY($2) AND read_at IS NULL`, auth.Current(c).UserID, req.IDs)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": tag.RowsAffected()})
}

func (h *handler) markAllRead(c *gin.Context) {
	tag, err := h.svc.app.DB.Exec(c.Request.Context(),
		`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": tag.RowsAffected()})
}
