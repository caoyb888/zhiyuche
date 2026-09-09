package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

// accountSelect expects $1 = month start, $2 = month end (for month_spent),
// $3 = tenant; further conditions continue from $4.
const accountSelect = `
	SELECT a.id, a.tenant_id, a.level, a.owner_id,
	       COALESCE(CASE a.level WHEN 'enterprise' THEN t.name WHEN 'department' THEN d.name ELSE u.name END, '') AS owner_name,
	       CASE a.level WHEN 'department' THEN pd.name WHEN 'employee' THEN ud.name END AS owner_sub,
	       a.balance::float8 AS balance, a.credit_limit::float8 AS credit_limit, a.monthly_budget::float8 AS monthly_budget,
	       COALESCE((SELECT sum(abs(x.amount)) FROM account_transactions x
	                 WHERE x.account_id = a.id AND x.type IN ('trip','charge','penalty')
	                   AND x.created_at >= $1 AND x.created_at < $2), 0)::float8 AS month_spent,
	       a.status, a.created_at, a.updated_at
	FROM accounts a
	LEFT JOIN tenants t ON a.level = 'enterprise' AND t.id = a.owner_id
	LEFT JOIN departments d ON a.level = 'department' AND d.id = a.owner_id
	LEFT JOIN departments pd ON pd.id = d.parent_id
	LEFT JOIN users u ON a.level = 'employee' AND u.id = a.owner_id
	LEFT JOIN departments ud ON ud.id = u.dept_id
	WHERE a.tenant_id = $3`

// txnSelect: $1 = tenant; further conditions continue from $2.
const txnSelect = `
	SELECT x.id, x.tenant_id, x.account_id, a.level AS account_level,
	       COALESCE(CASE a.level WHEN 'enterprise' THEN t.name WHEN 'department' THEN d.name ELSE u.name END, '') AS account_name,
	       x.type, x.amount::float8 AS amount, x.balance_after::float8 AS balance_after, x.ref_type, x.ref_id,
	       CASE x.ref_type
	         WHEN 'trip' THEN (SELECT tr.trip_no FROM trips tr WHERE tr.id::text = x.ref_id)
	         WHEN 'charge_transaction' THEN (SELECT ct.tx_no FROM charge_transactions ct WHERE ct.id::text = x.ref_id)
	       END AS ref_no,
	       x.remark, x.created_by, cu.name AS created_by_name, x.created_at
	FROM account_transactions x
	JOIN accounts a ON a.id = x.account_id
	LEFT JOIN tenants t ON a.level = 'enterprise' AND t.id = a.owner_id
	LEFT JOIN departments d ON a.level = 'department' AND d.id = a.owner_id
	LEFT JOIN users u ON a.level = 'employee' AND u.id = a.owner_id
	LEFT JOIN users cu ON cu.id = x.created_by
	WHERE x.tenant_id = $1`

func (s *Service) monthArgs(tenantID uuid.UUID) []any {
	ms, me := monthBounds(s.now())
	return []any{ms, me, tenantID}
}

func (s *Service) GetAccount(ctx context.Context, tenantID, id uuid.UUID) (*Account, error) {
	var a Account
	err := pgxscan.Get(ctx, s.app.DB, &a, accountSelect+` AND a.id = $4`, append(s.monthArgs(tenantID), id)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("账户不存在")
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// findAccount returns the account by (level, owner) or nil.
func (s *Service) findAccount(ctx context.Context, tenantID uuid.UUID, tg target) (*Account, error) {
	var a Account
	err := pgxscan.Get(ctx, s.app.DB, &a, accountSelect+` AND a.level = $4 AND a.owner_id = $5`, append(s.monthArgs(tenantID), tg.Level, tg.Owner)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// EnterpriseAccount returns the tenant's enterprise account, creating it on first access.
func (s *Service) EnterpriseAccount(ctx context.Context, tenantID uuid.UUID) (*Account, error) {
	var id uuid.UUID
	err := s.withTx(ctx, func(tx pgx.Tx) (err error) {
		id, err = s.ensureAccount(ctx, tx, tenantID, target{LevelEnterprise, tenantID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.GetAccount(ctx, tenantID, id)
}

// accountSortable maps sort keys onto columns of the wrapped accountSelect.
var accountSortable = map[string]string{
	"balance": "balance", "level": "level", "created_at": "created_at", "credit_limit": "credit_limit",
	"monthly_budget": "monthly_budget", "month_spent": "month_spent", "owner_name": "owner_name",
}

func (s *Service) ListAccounts(ctx context.Context, tenantID uuid.UUID, q AccountListQuery, pg pagination.Query) (pagination.Page[Account], error) {
	args := s.monthArgs(tenantID)
	where := []string{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if q.Level != "" {
		add("a.level = $%d", q.Level)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(t.name ILIKE $%[1]d OR d.name ILIKE $%[1]d OR u.name ILIKE $%[1]d OR u.username ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Negative {
		where = append(where, "a.balance < 0")
	}
	cond := ""
	if len(where) > 0 {
		cond = " AND " + strings.Join(where, " AND ")
	}
	var total int64
	if err := s.app.DB.QueryRow(ctx, `SELECT count(*) FROM (`+accountSelect+cond+`) q`, args...).Scan(&total); err != nil {
		return pagination.Page[Account]{}, err
	}
	order := "CASE level WHEN 'enterprise' THEN 0 WHEN 'department' THEN 1 ELSE 2 END, owner_name"
	if col, ok := accountSortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	rows := []Account{}
	err := pgxscan.Select(ctx, s.app.DB, &rows,
		fmt.Sprintf("SELECT * FROM (%s%s) q ORDER BY %s LIMIT $%d OFFSET $%d", accountSelect, cond, order, len(args)-1, len(args)), args...)
	if err != nil {
		return pagination.Page[Account]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func (s *Service) UpdateAccount(ctx context.Context, tenantID, id uuid.UUID, req AccountUpdateRequest) (*Account, *Account, error) {
	before, err := s.GetAccount(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.CreditLimit != nil {
		set("credit_limit", round2(*req.CreditLimit))
	}
	if req.MonthlyBudget != nil {
		set("monthly_budget", round2(*req.MonthlyBudget))
	}
	if req.Status != nil {
		set("status", *req.Status)
	}
	if len(sets) > 0 {
		if _, err := s.app.DB.Exec(ctx, fmt.Sprintf(`UPDATE accounts SET %s WHERE tenant_id = $1 AND id = $2`, strings.Join(sets, ", ")), args...); err != nil {
			return nil, nil, err
		}
	}
	after, err := s.GetAccount(ctx, tenantID, id)
	return before, after, err
}

func (s *Service) GetTransaction(ctx context.Context, tenantID uuid.UUID, id int64) (*Transaction, error) {
	var t Transaction
	err := pgxscan.Get(ctx, s.app.DB, &t, txnSelect+` AND x.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("流水不存在")
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Recharge credits the enterprise account (offline payment registered by finance).
func (s *Service) Recharge(ctx context.Context, tenantID uuid.UUID, req RechargeRequest, actor uuid.UUID) (*Transaction, error) {
	var p *posted
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		id, err := s.ensureAccount(ctx, tx, tenantID, target{LevelEnterprise, tenantID})
		if err != nil {
			return err
		}
		acc, err := s.lockAccount(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		remark := strings.TrimSpace(req.Remark)
		if remark == "" {
			remark = "企业账户充值"
		}
		p, err = s.post(ctx, tx, acc, posting{typ: TxnRecharge, amount: req.Amount, remark: remark, createdBy: &actor})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.publish(tenantID, p.Account, p.BalanceAfter)
	return s.GetTransaction(ctx, tenantID, p.TxnID)
}

// Allocate moves budget one level down (enterprise → department → employee).
func (s *Service) Allocate(ctx context.Context, tenantID, fromID uuid.UUID, req AllocateRequest, actor uuid.UUID) (*AllocateResponse, error) {
	if req.ToAccountID == nil && (req.ToLevel == "" || req.ToOwnerID == nil) {
		return nil, httpx.BadRequest("须指定 to_account_id 或 to_level + to_owner_id")
	}
	var out, in *posted
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		from, err := s.lockAccount(ctx, tx, tenantID, fromID)
		if err != nil {
			return err
		}
		if from.Status != "active" {
			return httpx.Conflict("源账户已冻结")
		}
		var toID uuid.UUID
		if req.ToAccountID != nil {
			var level string
			err := tx.QueryRow(ctx, `SELECT level FROM accounts WHERE tenant_id = $1 AND id = $2`, tenantID, *req.ToAccountID).Scan(&level)
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.NotFound("目标账户不存在")
			}
			if err != nil {
				return err
			}
			if !canAllocate(from.Level, level) {
				return httpx.BadRequest("只允许 企业→部门、部门→员工 划拨")
			}
			toID = *req.ToAccountID
		} else {
			if !canAllocate(from.Level, req.ToLevel) {
				return httpx.BadRequest("只允许 企业→部门、部门→员工 划拨")
			}
			if toID, err = s.ensureAccount(ctx, tx, tenantID, target{req.ToLevel, *req.ToOwnerID}); err != nil {
				return err
			}
		}
		if toID == from.ID {
			return httpx.BadRequest("不能划拨给自己")
		}
		if !sufficient(from.Balance, from.CreditLimit, req.Amount) {
			return httpx.Conflict(fmt.Sprintf("余额不足：可用 ¥%.2f（含透支额度），需要 ¥%.2f", from.Balance+from.CreditLimit, req.Amount))
		}
		to, err := s.lockAccount(ctx, tx, tenantID, toID)
		if err != nil {
			return err
		}
		remark := strings.TrimSpace(req.Remark)
		if remark == "" {
			remark = "额度划拨"
		}
		if out, err = s.post(ctx, tx, from, posting{typ: TxnAllocateOut, amount: -req.Amount, refType: RefAccount, refID: to.ID.String(), remark: remark, createdBy: &actor}); err != nil {
			return err
		}
		in, err = s.post(ctx, tx, to, posting{typ: TxnAllocateIn, amount: req.Amount, refType: RefAccount, refID: from.ID.String(), remark: remark, createdBy: &actor})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.publish(tenantID, out.Account, out.BalanceAfter)
	s.publish(tenantID, in.Account, in.BalanceAfter)
	fromTxn, err := s.GetTransaction(ctx, tenantID, out.TxnID)
	if err != nil {
		return nil, err
	}
	toTxn, err := s.GetTransaction(ctx, tenantID, in.TxnID)
	if err != nil {
		return nil, err
	}
	return &AllocateResponse{From: *fromTxn, To: *toTxn}, nil
}

// Adjust posts a signed manual correction (adjust / refund).
func (s *Service) Adjust(ctx context.Context, tenantID, id uuid.UUID, req AdjustRequest, actor uuid.UUID) (*Transaction, error) {
	if req.Amount == 0 {
		return nil, httpx.BadRequest("amount 不能为 0")
	}
	typ := req.Type
	if typ == "" {
		typ = TxnAdjust
	}
	var p *posted
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		acc, err := s.lockAccount(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		p, err = s.post(ctx, tx, acc, posting{typ: typ, amount: req.Amount, refType: strings.TrimSpace(req.RefType), refID: strings.TrimSpace(req.RefID), remark: strings.TrimSpace(req.Remark), createdBy: &actor})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.afterDebit(ctx, tenantID, p)
	return s.GetTransaction(ctx, tenantID, p.TxnID)
}

func (s *Service) listTxns(ctx context.Context, tenantID uuid.UUID, cond string, args []any, pg pagination.Query) (pagination.Page[Transaction], error) {
	var total int64
	if err := s.app.DB.QueryRow(ctx, `SELECT count(*) FROM (`+txnSelect+cond+`) q`, args...).Scan(&total); err != nil {
		return pagination.Page[Transaction]{}, err
	}
	args = append(args, pg.Limit(), pg.Offset())
	rows := []Transaction{}
	err := pgxscan.Select(ctx, s.app.DB, &rows,
		fmt.Sprintf("%s%s ORDER BY x.created_at DESC, x.id DESC LIMIT $%d OFFSET $%d", txnSelect, cond, len(args)-1, len(args)), args...)
	if err != nil {
		return pagination.Page[Transaction]{}, err
	}
	return pagination.NewPage(rows, total, pg), nil
}

func txnFilters(q TxnListQuery, args *[]any) []string {
	where := []string{}
	add := func(cond string, v any) {
		*args = append(*args, v)
		where = append(where, fmt.Sprintf(cond, len(*args)))
	}
	if q.Level != "" {
		add("a.level = $%d", q.Level)
	}
	if q.Type != "" {
		add("x.type = $%d", q.Type)
	}
	if q.From != nil {
		add("x.created_at >= $%d", *q.From)
	}
	if q.To != nil {
		add("x.created_at < $%d", *q.To)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(x.remark ILIKE $%[1]d OR x.ref_id ILIKE $%[1]d OR t.name ILIKE $%[1]d OR d.name ILIKE $%[1]d OR u.name ILIKE $%[1]d)", "%"+kw+"%")
	}
	return where
}

// ListAccountTransactions is the statement of one account.
func (s *Service) ListAccountTransactions(ctx context.Context, tenantID, accountID uuid.UUID, q TxnListQuery, pg pagination.Query) (pagination.Page[Transaction], error) {
	if _, err := s.GetAccount(ctx, tenantID, accountID); err != nil {
		return pagination.Page[Transaction]{}, err
	}
	args := []any{tenantID, accountID}
	where := append([]string{"x.account_id = $2"}, txnFilters(q, &args)...)
	return s.listTxns(ctx, tenantID, " AND "+strings.Join(where, " AND "), args, pg)
}

// ListTransactions is the tenant-wide ledger.
func (s *Service) ListTransactions(ctx context.Context, tenantID uuid.UUID, q TxnListQuery, pg pagination.Query) (pagination.Page[Transaction], error) {
	args := []any{tenantID}
	where := txnFilters(q, &args)
	cond := ""
	if len(where) > 0 {
		cond = " AND " + strings.Join(where, " AND ")
	}
	return s.listTxns(ctx, tenantID, cond, args, pg)
}

// MyAccount is the caller's employee account (virtual when not yet created) + last 20 movements.
func (s *Service) MyAccount(ctx context.Context, tenantID, userID uuid.UUID) (*MyAccount, error) {
	acc, err := s.findAccount(ctx, tenantID, target{LevelEmployee, userID})
	if err != nil {
		return nil, err
	}
	out := &MyAccount{Transactions: []Transaction{}}
	if acc == nil {
		var name string
		var dept *string
		_ = s.app.DB.QueryRow(ctx, `SELECT u.name, d.name FROM users u LEFT JOIN departments d ON d.id = u.dept_id WHERE u.id = $1`, userID).Scan(&name, &dept)
		out.Account = Account{TenantID: tenantID, Level: LevelEmployee, OwnerID: userID, OwnerName: name, OwnerSub: dept, Status: "active"}
		return out, nil
	}
	out.Account, out.Exists = *acc, true
	page, err := s.ListAccountTransactions(ctx, tenantID, acc.ID, TxnListQuery{}, pagination.Query{Page: 1, PageSize: 20})
	if err != nil {
		return nil, err
	}
	out.Transactions = page.Items
	return out, nil
}

// ---- tree ----

type deptRow struct {
	ID            uuid.UUID  `db:"id"`
	ParentID      *uuid.UUID `db:"parent_id"`
	Name          string     `db:"name"`
	Sort          int        `db:"sort"`
	MonthlyBudget float64    `db:"monthly_budget"`
	ParentName    *string    `db:"parent_name"`
}

type userRow struct {
	ID       uuid.UUID  `db:"id"`
	DeptID   *uuid.UUID `db:"dept_id"`
	Name     string     `db:"name"`
	DeptName *string    `db:"dept_name"`
}

// AccountTree: enterprise → departments (tree) → active employees. Employees
// without a department hang directly under the enterprise node.
func (s *Service) AccountTree(ctx context.Context, tenantID uuid.UUID) (*AccountNode, error) {
	ent, err := s.EnterpriseAccount(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	if err := pgxscan.Select(ctx, s.app.DB, &accounts, accountSelect, s.monthArgs(tenantID)...); err != nil {
		return nil, err
	}
	byOwner := map[target]Account{}
	for _, a := range accounts {
		byOwner[target{a.Level, a.OwnerID}] = a
	}
	var depts []deptRow
	if err := pgxscan.Select(ctx, s.app.DB, &depts, `
		SELECT d.id, d.parent_id, d.name, d.sort, d.monthly_budget::float8 AS monthly_budget, p.name AS parent_name
		FROM departments d LEFT JOIN departments p ON p.id = d.parent_id
		WHERE d.tenant_id = $1 AND d.deleted_at IS NULL ORDER BY d.sort, d.name, d.created_at`, tenantID); err != nil {
		return nil, err
	}
	var users []userRow
	if err := pgxscan.Select(ctx, s.app.DB, &users, `
		SELECT u.id, u.dept_id, u.name, d.name AS dept_name FROM users u LEFT JOIN departments d ON d.id = u.dept_id
		WHERE u.tenant_id = $1 AND u.deleted_at IS NULL AND u.status = 'active' AND NOT u.is_super ORDER BY u.name`, tenantID); err != nil {
		return nil, err
	}
	node := func(level string, owner uuid.UUID, name string, sub *string, budget float64) *AccountNode {
		if a, ok := byOwner[target{level, owner}]; ok {
			return &AccountNode{Account: a, Exists: true, Children: []*AccountNode{}}
		}
		return &AccountNode{Account: Account{TenantID: tenantID, Level: level, OwnerID: owner, OwnerName: name, OwnerSub: sub, MonthlyBudget: budget, Status: "active"}, Children: []*AccountNode{}}
	}
	root := &AccountNode{Account: *ent, Exists: true, Children: []*AccountNode{}}
	deptNodes := map[uuid.UUID]*AccountNode{}
	for _, d := range depts {
		deptNodes[d.ID] = node(LevelDepartment, d.ID, d.Name, d.ParentName, d.MonthlyBudget)
	}
	for _, d := range depts { // parents come before children only by luck; attach after all exist
		n := deptNodes[d.ID]
		if d.ParentID != nil {
			if p, ok := deptNodes[*d.ParentID]; ok {
				p.Children = append(p.Children, n)
				continue
			}
		}
		root.Children = append(root.Children, n)
	}
	for _, u := range users {
		n := node(LevelEmployee, u.ID, u.Name, u.DeptName, 0)
		if u.DeptID != nil {
			if p, ok := deptNodes[*u.DeptID]; ok {
				p.Children = append(p.Children, n)
				continue
			}
		}
		root.Children = append(root.Children, n)
	}
	return root, nil
}
