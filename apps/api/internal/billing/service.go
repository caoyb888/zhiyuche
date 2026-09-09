package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/notify"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Service implements rules, accounts, hooks and settlements. One instance is
// shared by the HTTP handler and the trip / charging hooks.
type Service struct {
	app    *app.App
	notify *notify.Service
	now    func() time.Time // injectable for tests
}

func NewService(a *app.App) *Service {
	return &Service{app: a, notify: notify.NewService(a), now: time.Now}
}

// ---- account primitives (all inside the caller's transaction) ----

// accountRow is the locked snapshot used while posting.
type accountRow struct {
	ID          uuid.UUID `db:"id"`
	TenantID    uuid.UUID `db:"tenant_id"`
	Level       string    `db:"level"`
	OwnerID     uuid.UUID `db:"owner_id"`
	Balance     float64   `db:"balance"`
	CreditLimit float64   `db:"credit_limit"`
	Status      string    `db:"status"`
}

// ensureAccount returns the account id for (tenant, level, owner), creating
// the row on first use. Department accounts start with the department's
// monthly budget. The owner must exist in the tenant (enterprise: the tenant).
func (s *Service) ensureAccount(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, tg target) (uuid.UUID, error) {
	var id uuid.UUID
	var budgetSQL string
	switch tg.Level {
	case LevelEnterprise:
		if tg.Owner != tenantID {
			return uuid.Nil, httpx.BadRequest("企业账户 owner 须为租户")
		}
		budgetSQL = "0"
	case LevelDepartment:
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM departments WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, tg.Owner).Scan(&n); err != nil {
			return uuid.Nil, err
		}
		if n == 0 {
			return uuid.Nil, httpx.BadRequest("部门不存在")
		}
		budgetSQL = "COALESCE((SELECT monthly_budget FROM departments WHERE id = $3), 0)"
	case LevelEmployee:
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, tg.Owner).Scan(&n); err != nil {
			return uuid.Nil, err
		}
		if n == 0 {
			return uuid.Nil, httpx.BadRequest("用户不存在")
		}
		budgetSQL = "0"
	default:
		return uuid.Nil, httpx.BadRequest("账户级别不合法")
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO accounts (tenant_id, level, owner_id, monthly_budget) VALUES ($1, $2, $3, %s)
		ON CONFLICT (tenant_id, level, owner_id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id
		RETURNING id`, budgetSQL), tenantID, tg.Level, tg.Owner).Scan(&id)
	return id, err
}

// lockAccount reads the account FOR UPDATE.
func (s *Service) lockAccount(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (*accountRow, error) {
	var r accountRow
	err := pgxscan.Get(ctx, tx, &r, `
		SELECT id, tenant_id, level, owner_id, balance::float8 AS balance, credit_limit::float8 AS credit_limit, status
		FROM accounts WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("账户不存在")
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// posting is one signed balance movement.
type posting struct {
	typ       string
	amount    float64 // signed: credit > 0, debit < 0
	refType   string
	refID     string
	remark    string
	createdBy *uuid.UUID
}

// posted is the outcome of post.
type posted struct {
	TxnID        int64
	BalanceAfter float64
	Overdrawn    bool
	Account      accountRow
}

// post applies the movement to a locked account and writes the transaction.
func (s *Service) post(ctx context.Context, tx pgx.Tx, acc *accountRow, p posting) (*posted, error) {
	amount := round2(p.amount)
	after := round2(acc.Balance + amount)
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance = $2 WHERE id = $1`, acc.ID, after); err != nil {
		return nil, err
	}
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO account_transactions (tenant_id, account_id, type, amount, balance_after, ref_type, ref_id, remark, created_by)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9) RETURNING id`,
		acc.TenantID, acc.ID, p.typ, amount, after, p.refType, p.refID, p.remark, p.createdBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	acc.Balance = after
	return &posted{TxnID: id, BalanceAfter: after, Overdrawn: overdrawn(after, acc.CreditLimit), Account: *acc}, nil
}

// debit resolves + locks the attributed account and posts a negative amount
// (0 → no transaction, account still ensured). Overdraft never blocks.
func (s *Service) debit(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, tg target, p posting) (*posted, error) {
	id, err := s.ensureAccount(ctx, tx, tenantID, tg)
	if err != nil {
		return nil, err
	}
	acc, err := s.lockAccount(ctx, tx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p.amount == 0 {
		return &posted{BalanceAfter: acc.Balance, Account: *acc}, nil
	}
	if p.amount > 0 {
		p.amount = -p.amount
	}
	return s.post(ctx, tx, acc, p)
}

// ---- after-commit side effects ----

// publish pushes account.updated to the tenant.
func (s *Service) publish(tenantID uuid.UUID, acc accountRow, balance float64) {
	s.app.Hub.Publish(tenantID, ws.Event{Type: EvAccountUpdated, Data: map[string]any{
		"account_id": acc.ID, "level": acc.Level, "owner_id": acc.OwnerID, "balance": balance,
	}})
}

// ownerName resolves the display name of an account owner.
func (s *Service) ownerName(ctx context.Context, acc accountRow) string {
	var name string
	var err error
	switch acc.Level {
	case LevelEnterprise:
		err = s.app.DB.QueryRow(ctx, `SELECT name FROM tenants WHERE id = $1`, acc.OwnerID).Scan(&name)
	case LevelDepartment:
		err = s.app.DB.QueryRow(ctx, `SELECT name FROM departments WHERE id = $1`, acc.OwnerID).Scan(&name)
	default:
		err = s.app.DB.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, acc.OwnerID).Scan(&name)
	}
	if err != nil {
		return ""
	}
	return name
}

// notifyOverdraft tells the account's responsible person (employee → self,
// department → leader) that the credit limit is exhausted.
func (s *Service) notifyOverdraft(ctx context.Context, tenantID uuid.UUID, acc accountRow, balance float64) {
	var to uuid.UUID
	switch acc.Level {
	case LevelEmployee:
		to = acc.OwnerID
	case LevelDepartment:
		var leader *uuid.UUID
		if err := s.app.DB.QueryRow(ctx, `SELECT leader_user_id FROM departments WHERE id = $1`, acc.OwnerID).Scan(&leader); err != nil || leader == nil {
			return
		}
		to = *leader
	default:
		return
	}
	label := accountLabel(acc.Level, s.ownerName(ctx, acc))
	err := s.notify.Send(ctx, tenantID, []uuid.UUID{to}, notify.Message{
		Type:    notify.TypeSystem,
		Title:   "账户余额不足",
		Content: fmt.Sprintf("%s 余额 ¥%.2f 已超出透支额度 ¥%.2f，请及时充值或划拨额度", label, balance, acc.CreditLimit),
		RefType: RefAccount, RefID: acc.ID.String(),
	})
	if err != nil {
		s.app.Log.Warn().Err(err).Str("account", acc.ID.String()).Msg("overdraft notification failed")
	}
}

// afterDebit runs the push + notifications of a committed debit.
func (s *Service) afterDebit(ctx context.Context, tenantID uuid.UUID, p *posted) {
	if p == nil || p.TxnID == 0 {
		return
	}
	s.publish(tenantID, p.Account, p.BalanceAfter)
	if p.Overdrawn {
		s.notifyOverdraft(ctx, tenantID, p.Account, p.BalanceAfter)
	}
}

// withTx runs fn in a transaction.
func (s *Service) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.app.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
