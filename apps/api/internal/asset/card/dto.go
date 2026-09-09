// Package card manages NFC cards (/assets/nfc-cards): issuing, binding a
// holder, loss reporting. A card uid is stored upper-case and is unique per tenant.
package card

import (
	"time"

	"github.com/google/uuid"
)

// Card is the API representation (contract schema Card).
type Card struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	TenantID  uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	CardUID   string     `json:"card_uid" db:"card_uid"`
	UserID    *uuid.UUID `json:"user_id" db:"user_id"`
	UserName  *string    `json:"user_name" db:"user_name"`
	Username  *string    `json:"username" db:"username"`
	Status    string     `json:"status" db:"status"`
	IssuedAt  *time.Time `json:"issued_at" db:"issued_at"`
	Remark    *string    `json:"remark" db:"remark"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// ListQuery mirrors the query string of GET /assets/nfc-cards.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=active lost disabled"`
	Bound   *bool  `form:"bound"`
}

type CreateRequest struct {
	CardUID  string     `json:"card_uid" binding:"required"`
	UserID   *uuid.UUID `json:"user_id"`
	IssuedAt *time.Time `json:"issued_at"`
	Remark   *string    `json:"remark" binding:"omitempty,max=500"`
}

type UpdateRequest struct {
	Status *string `json:"status" binding:"omitempty,oneof=active lost disabled"`
	Remark *string `json:"remark" binding:"omitempty,max=500"`
}

type BindRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}
