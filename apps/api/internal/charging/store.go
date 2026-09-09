package charging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type store struct{ db *pgxpool.Pool }

// ---- piles & connectors

const pileSelect = `
	SELECT p.id, p.tenant_id, p.pile_code, p.name, p.type, p.power_kw, p.connector_count, p.vendor, p.location,
	       p.lng, p.lat, p.status, p.last_heartbeat_at
	FROM charge_piles p
	WHERE p.deleted_at IS NULL`

// pileByCode resolves the OCPP identity. pile_code is unique per tenant only;
// with several tenants sharing a code the oldest archive wins (logged by the caller).
func (s *store) pileByCode(ctx context.Context, code string) (*PileLive, int, error) {
	var rows []PileLive
	if err := pgxscan.Select(ctx, s.db, &rows, pileSelect+` AND p.pile_code = $1 AND p.status <> 'disabled' ORDER BY p.created_at`, code); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, nil
	}
	return &rows[0], len(rows), nil
}

func (s *store) pile(ctx context.Context, tenantID, id uuid.UUID) (*PileLive, error) {
	var p PileLive
	err := pgxscan.Get(ctx, s.db, &p, pileSelect+` AND p.tenant_id = $1 AND p.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *store) pileAny(ctx context.Context, id uuid.UUID) (*PileLive, error) {
	var p PileLive
	err := pgxscan.Get(ctx, s.db, &p, pileSelect+` AND p.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *store) pilesByTenant(ctx context.Context, tenantID uuid.UUID) ([]PileLive, error) {
	var rows []PileLive
	err := pgxscan.Select(ctx, s.db, &rows, pileSelect+` AND p.tenant_id = $1 ORDER BY p.pile_code`, tenantID)
	if rows == nil {
		rows = []PileLive{}
	}
	return rows, err
}

func (s *store) connectors(ctx context.Context, pileIDs []uuid.UUID) (map[uuid.UUID][]Connector, error) {
	out := map[uuid.UUID][]Connector{}
	if len(pileIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `SELECT pile_id, connector_id, status, error_code, info, updated_at FROM pile_connectors WHERE pile_id = ANY($1) ORDER BY pile_id, connector_id`, pileIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid uuid.UUID
		var c Connector
		if err := rows.Scan(&pid, &c.ConnectorID, &c.Status, &c.ErrorCode, &c.Info, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], c)
	}
	return out, rows.Err()
}

func (s *store) connectorsOf(ctx context.Context, pileID uuid.UUID) ([]Connector, error) {
	m, err := s.connectors(ctx, []uuid.UUID{pileID})
	if err != nil {
		return nil, err
	}
	return m[pileID], nil
}

func (s *store) upsertConnector(ctx context.Context, pileID uuid.UUID, connectorID int, status string, errCode, info *string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO pile_connectors (pile_id, connector_id, status, error_code, info, updated_at) VALUES ($1,$2,$3,$4,$5,now())
		ON CONFLICT (pile_id, connector_id) DO UPDATE SET status = EXCLUDED.status, error_code = EXCLUDED.error_code, info = EXCLUDED.info, updated_at = now()`,
		pileID, connectorID, status, errCode, info)
	return err
}

func (s *store) setAllConnectors(ctx context.Context, pileID uuid.UUID, status string) error {
	_, err := s.db.Exec(ctx, `UPDATE pile_connectors SET status = $2, updated_at = now() WHERE pile_id = $1`, pileID, status)
	return err
}

// setPileStatus never overrides a disabled pile.
func (s *store) setPileStatus(ctx context.Context, pileID uuid.UUID, status string) error {
	_, err := s.db.Exec(ctx, `UPDATE charge_piles SET status = $2 WHERE id = $1 AND status <> 'disabled' AND deleted_at IS NULL`, pileID, status)
	return err
}

func (s *store) boot(ctx context.Context, pileID uuid.UUID, vendor string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE charge_piles SET vendor = COALESCE(NULLIF(vendor, ''), NULLIF($2, '')), last_heartbeat_at = now(),
		  status = CASE WHEN status = 'disabled' THEN status ELSE 'available' END
		WHERE id = $1 AND deleted_at IS NULL`, pileID, vendor)
	return err
}

func (s *store) heartbeat(ctx context.Context, pileID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE charge_piles SET last_heartbeat_at = now() WHERE id = $1 AND deleted_at IS NULL`, pileID)
	return err
}

// sweepStale flips online piles whose heartbeat is older than cutoff to offline and returns them.
func (s *store) sweepStale(ctx context.Context, cutoff time.Time) ([]PileLive, error) {
	var rows []PileLive
	err := pgxscan.Select(ctx, s.db, &rows, `
		UPDATE charge_piles p SET status = 'offline'
		WHERE p.deleted_at IS NULL AND p.status NOT IN ('offline','disabled') AND (p.last_heartbeat_at IS NULL OR p.last_heartbeat_at < $1)
		RETURNING p.id, p.tenant_id, p.pile_code, p.name, p.type, p.power_kw, p.connector_count, p.vendor, p.location, p.lng, p.lat, p.status, p.last_heartbeat_at`, cutoff)
	return rows, err
}

// ---- cards / users / vehicles

type cardRow struct {
	ID     uuid.UUID  `db:"id"`
	UserID *uuid.UUID `db:"user_id"`
	Status string     `db:"status"`
}

func (s *store) cardByUID(ctx context.Context, tenantID uuid.UUID, uid string) (*cardRow, error) {
	var c cardRow
	err := pgxscan.Get(ctx, s.db, &c, `SELECT id, user_id, status FROM nfc_cards WHERE tenant_id = $1 AND upper(card_uid) = upper($2) AND deleted_at IS NULL`, tenantID, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *store) firstActiveCard(ctx context.Context, tenantID, userID uuid.UUID) (string, error) {
	var uid string
	err := s.db.QueryRow(ctx, `SELECT card_uid FROM nfc_cards WHERE tenant_id = $1 AND user_id = $2 AND status = 'active' AND deleted_at IS NULL ORDER BY created_at LIMIT 1`, tenantID, userID).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return uid, err
}

type userRow struct {
	ID     uuid.UUID  `db:"id"`
	DeptID *uuid.UUID `db:"dept_id"`
	Name   string     `db:"name"`
	Status string     `db:"status"`
}

func (s *store) user(ctx context.Context, tenantID, id uuid.UUID) (*userRow, error) {
	var u userRow
	err := pgxscan.Get(ctx, s.db, &u, `SELECT id, dept_id, name, status FROM users WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// tenantAdmins returns the active users holding the built-in tenant_admin role.
func (s *store) tenantAdmins(ctx context.Context, tenantID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := pgxscan.Select(ctx, s.db, &ids, `
		SELECT DISTINCT u.id FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE u.tenant_id = $1 AND u.deleted_at IS NULL AND u.status = 'active' AND r.code = 'tenant_admin'`, tenantID)
	return ids, err
}

type vehicleRow struct {
	ID         uuid.UUID  `db:"id"`
	TenantID   uuid.UUID  `db:"tenant_id"`
	PlateNo    string     `db:"plate_no"`
	Status     string     `db:"status"`
	BatteryKwh float64    `db:"battery_kwh"`
	HomeDeptID *uuid.UUID `db:"home_dept_id"`
}

func (s *store) vehicle(ctx context.Context, id uuid.UUID) (*vehicleRow, error) {
	var v vehicleRow
	err := pgxscan.Get(ctx, s.db, &v, `SELECT id, tenant_id, plate_no, status, battery_kwh, home_dept_id FROM vehicles WHERE id = $1 AND deleted_at IS NULL`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// nearbyVehicles lists the tenant's vehicles that reported since `since`, are
// within a coarse box (~1 km) around the point and are not already charging.
// Exact distance is computed in Go (see PickVehicle).
func (s *store) nearbyVehicles(ctx context.Context, tenantID uuid.UUID, lng, lat float64, since time.Time) ([]NearbyVehicle, error) {
	rows, err := s.db.Query(ctx, `
		SELECT vs.vehicle_id, vs.lng, vs.lat, vs.last_telemetry_at
		FROM vehicle_status vs JOIN vehicles v ON v.id = vs.vehicle_id AND v.deleted_at IS NULL
		WHERE vs.tenant_id = $1 AND vs.lng IS NOT NULL AND vs.lat IS NOT NULL AND vs.last_telemetry_at >= $2
		  AND v.status <> 'disabled' AND abs(vs.lng - $3) < 0.012 AND abs(vs.lat - $4) < 0.01
		  AND NOT EXISTS (SELECT 1 FROM charge_transactions ct WHERE ct.vehicle_id = vs.vehicle_id AND ct.status = 'charging')`,
		tenantID, since, lng, lat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NearbyVehicle
	for rows.Next() {
		var n NearbyVehicle
		var vlng, vlat float64
		var seen time.Time
		if err := rows.Scan(&n.VehicleID, &vlng, &vlat, &seen); err != nil {
			return nil, err
		}
		n.DistanceM = DistanceM(lng, lat, vlng, vlat)
		n.LastSeen = seen
		out = append(out, n)
	}
	return out, rows.Err()
}

// recentTripVehicle returns the vehicle of the user's latest completed trip ended after `since`.
func (s *store) recentTripVehicle(ctx context.Context, tenantID, userID uuid.UUID, since time.Time) (*uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT vehicle_id FROM trips WHERE tenant_id = $1 AND driver_id = $2 AND status = 'completed' AND end_at >= $3
		ORDER BY end_at DESC LIMIT 1`, tenantID, userID, since).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// telemetrySOCAt returns the vehicle's SOC from the last telemetry point at or before `at`.
func (s *store) telemetrySOCAt(ctx context.Context, vehicleID uuid.UUID, at time.Time) (*float64, error) {
	var soc *float64
	err := s.db.QueryRow(ctx, `SELECT soc FROM vehicle_telemetry WHERE vehicle_id = $1 AND ts <= $2 AND soc IS NOT NULL ORDER BY ts DESC LIMIT 1`, vehicleID, at).Scan(&soc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return soc, err
}

// ---- transactions

// txRow is the flat scan target of the transaction list/detail query.
type txRow struct {
	ID           uuid.UUID  `db:"id"`
	TenantID     uuid.UUID  `db:"tenant_id"`
	TxNo         string     `db:"tx_no"`
	PileID       uuid.UUID  `db:"pile_id"`
	PileCode     string     `db:"pile_code"`
	PileName     string     `db:"pile_name"`
	ConnectorID  int        `db:"connector_id"`
	OcppTxID     int        `db:"ocpp_tx_id"`
	IDTag        string     `db:"id_tag"`
	CardID       *uuid.UUID `db:"card_id"`
	UserID       *uuid.UUID `db:"user_id"`
	DeptID       *uuid.UUID `db:"dept_id"`
	VehicleID    *uuid.UUID `db:"vehicle_id"`
	BindMethod   *string    `db:"bind_method"`
	Status       string     `db:"status"`
	StartAt      time.Time  `db:"start_at"`
	EndAt        *time.Time `db:"end_at"`
	MeterStart   float64    `db:"meter_start"`
	MeterStop    *float64   `db:"meter_stop"`
	Kwh          *float64   `db:"kwh"`
	UnitPrice    *float64   `db:"unit_price"`
	Cost         *float64   `db:"cost"`
	StopReason   *string    `db:"stop_reason"`
	BmsSocStart  *float64   `db:"bms_soc_start"`
	BmsSocEnd    *float64   `db:"bms_soc_end"`
	BmsKwhEst    *float64   `db:"bms_kwh_est"`
	DeviationPct *float64   `db:"deviation_pct"`
	ReviewStatus string     `db:"review_status"`
	ReviewNote   *string    `db:"review_note"`
	ReviewedBy   *uuid.UUID `db:"reviewed_by"`
	ReviewedAt   *time.Time `db:"reviewed_at"`
	Attribution  *string    `db:"attribution"`
	AccountID    *uuid.UUID `db:"account_id"`
	AccountTxnID *int64     `db:"account_txn_id"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`

	UserName     *string `db:"user_name"`
	UserUsername *string `db:"user_username"`
	UserPhone    *string `db:"user_phone"`
	UserDeptName *string `db:"user_dept_name"`
	DeptName     *string `db:"dept_name"`

	PlateNo             *string    `db:"plate_no"`
	VehicleBrand        *string    `db:"vehicle_brand"`
	VehicleModel        *string    `db:"vehicle_model"`
	VehicleStatus       *string    `db:"vehicle_status"`
	VehicleHomeDeptID   *uuid.UUID `db:"vehicle_home_dept_id"`
	VehicleHomeDeptName *string    `db:"vehicle_home_dept_name"`
	VehicleSOC          *float64   `db:"vehicle_soc"`
	VehicleRangeKm      *float64   `db:"vehicle_range_km"`
	VehicleBatteryKwh   *float64   `db:"vehicle_battery_kwh"`

	ReviewedByName *string  `db:"reviewed_by_name"`
	LastWh         *float64 `db:"last_wh"`
	LastPowerKw    *float64 `db:"last_power_kw"`
	LastSOC        *float64 `db:"last_soc"`
}

const txSelect = `
	SELECT t.id, t.tenant_id, t.tx_no, t.pile_id, p.pile_code, p.name AS pile_name, t.connector_id, t.ocpp_tx_id, t.id_tag,
	       t.card_id, t.user_id, t.dept_id, t.vehicle_id, t.bind_method, t.status, t.start_at, t.end_at, t.meter_start, t.meter_stop,
	       t.kwh, t.unit_price, t.cost, t.stop_reason, t.bms_soc_start, t.bms_soc_end, t.bms_kwh_est, t.deviation_pct,
	       t.review_status, t.review_note, t.reviewed_by, t.reviewed_at, t.attribution, t.account_id, t.account_txn_id, t.created_at, t.updated_at,
	       u.name AS user_name, u.username AS user_username, u.phone AS user_phone, ud.name AS user_dept_name, d.name AS dept_name,
	       v.plate_no, v.brand AS vehicle_brand, v.model AS vehicle_model, v.status AS vehicle_status,
	       v.home_dept_id AS vehicle_home_dept_id, vd.name AS vehicle_home_dept_name, vs.soc AS vehicle_soc, vs.range_km AS vehicle_range_km,
	       v.battery_kwh AS vehicle_battery_kwh,
	       r.name AS reviewed_by_name, lm.wh AS last_wh, lm.power_kw AS last_power_kw, lm.soc AS last_soc
	FROM charge_transactions t
	JOIN charge_piles p ON p.id = t.pile_id
	LEFT JOIN users u ON u.id = t.user_id
	LEFT JOIN departments ud ON ud.id = u.dept_id
	LEFT JOIN departments d ON d.id = t.dept_id
	LEFT JOIN vehicles v ON v.id = t.vehicle_id
	LEFT JOIN departments vd ON vd.id = v.home_dept_id
	LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
	LEFT JOIN users r ON r.id = t.reviewed_by
	LEFT JOIN LATERAL (SELECT m.wh, m.power_kw, m.soc FROM charge_meter_values m WHERE m.tx_id = t.id ORDER BY m.ts DESC LIMIT 1) lm ON true
	WHERE t.tenant_id = $1`

var sortable = map[string]string{
	"start_at": "t.start_at", "end_at": "t.end_at", "kwh": "t.kwh", "cost": "t.cost", "tx_no": "t.tx_no", "status": "t.status",
	"deviation_pct": "t.deviation_pct",
}

type listFilter struct {
	ListQuery
	from *time.Time
	to   *time.Time
}

func (s *store) listCond(f listFilter) (string, []any) {
	where := []string{}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)+1)) // $1 is tenant
	}
	if f.Status != "" {
		add("t.status = $%d", f.Status)
	}
	if f.ReviewStatus != "" {
		add("t.review_status = $%d", f.ReviewStatus)
	}
	if f.PileID != "" {
		add("t.pile_id = $%d", uuid.MustParse(f.PileID))
	}
	if f.VehicleID != "" {
		add("t.vehicle_id = $%d", uuid.MustParse(f.VehicleID))
	}
	if f.UserID != "" {
		add("t.user_id = $%d", uuid.MustParse(f.UserID))
	}
	if f.DeptID != "" {
		add("t.dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", uuid.MustParse(f.DeptID))
	}
	if f.from != nil {
		add("t.start_at >= $%d", *f.from)
	}
	if f.to != nil {
		add("t.start_at <= $%d", *f.to)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		add("(t.tx_no ILIKE $%[1]d OR p.pile_code ILIKE $%[1]d OR p.name ILIKE $%[1]d OR v.plate_no ILIKE $%[1]d OR u.name ILIKE $%[1]d OR t.id_tag ILIKE $%[1]d)", "%"+kw+"%")
	}
	if len(where) == 0 {
		return "", args
	}
	return " AND " + strings.Join(where, " AND "), args
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, f listFilter, pg pagination.Query) ([]txRow, int64, error) {
	cond, extra := s.listCond(f)
	args := append([]any{tenantID}, extra...)
	var total int64
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM charge_transactions t JOIN charge_piles p ON p.id = t.pile_id
		LEFT JOIN vehicles v ON v.id = t.vehicle_id LEFT JOIN users u ON u.id = t.user_id
		WHERE t.tenant_id = $1`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "t.start_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
		order += " NULLS LAST"
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []txRow
	err := pgxscan.Select(ctx, s.db, &rows, fmt.Sprintf("%s%s ORDER BY %s, t.id LIMIT $%d OFFSET $%d", txSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*txRow, error) {
	var r txRow
	err := pgxscan.Get(ctx, s.db, &r, txSelect+` AND t.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *store) getAny(ctx context.Context, id uuid.UUID) (*txRow, error) {
	var tenantID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT tenant_id FROM charge_transactions WHERE id = $1`, id).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.get(ctx, tenantID, id)
}

// activeOn returns the charging transaction of a connector, if any.
func (s *store) activeOn(ctx context.Context, pileID uuid.UUID, connectorID int) (*txRow, error) {
	var tenantID uuid.UUID
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT tenant_id, id FROM charge_transactions WHERE pile_id = $1 AND connector_id = $2 AND status = 'charging'`, pileID, connectorID).Scan(&tenantID, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.get(ctx, tenantID, id)
}

// activeByPile returns the charging transactions of a pile keyed by connector.
func (s *store) activeByPiles(ctx context.Context, tenantID uuid.UUID, pileIDs []uuid.UUID) ([]txRow, error) {
	if len(pileIDs) == 0 {
		return nil, nil
	}
	var rows []txRow
	err := pgxscan.Select(ctx, s.db, &rows, txSelect+` AND t.status = 'charging' AND t.pile_id = ANY($2)`, tenantID, pileIDs)
	return rows, err
}

// byOcpp finds a pile's transaction by the transactionId the pile uses.
func (s *store) byOcpp(ctx context.Context, pileID uuid.UUID, ocppTxID int) (*txRow, error) {
	var tenantID uuid.UUID
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT tenant_id, id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, pileID, ocppTxID).Scan(&tenantID, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.get(ctx, tenantID, id)
}

type insertParams struct {
	tenantID    uuid.UUID
	pileID      uuid.UUID
	connectorID int
	idTag       string
	cardID      *uuid.UUID
	userID      *uuid.UUID
	deptID      *uuid.UUID
	vehicleID   *uuid.UUID
	bindMethod  string
	startAt     time.Time
	meterStart  float64
	socStart    *float64
	review      string // review_status at creation (pending when nothing could be bound)
	note        *string
}

// insert creates the transaction with a day-sequenced tx_no (advisory lock)
// and returns the pile-facing transactionId (sequence).
func (s *store) insert(ctx context.Context, p insertParams) (uuid.UUID, string, int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", 0, err
	}
	defer tx.Rollback(ctx)
	txNo, err := approval.NextNo(ctx, tx, "charge_transactions", "tx_no", "C", p.startAt, 3)
	if err != nil {
		return uuid.Nil, "", 0, err
	}
	var id uuid.UUID
	var ocppID int
	err = tx.QueryRow(ctx, `
		INSERT INTO charge_transactions (tenant_id, tx_no, pile_id, connector_id, id_tag, card_id, user_id, dept_id, vehicle_id, bind_method,
		  status, start_at, meter_start, bms_soc_start, review_status, review_note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'charging',$11,$12,$13,$14,$15) RETURNING id, ocpp_tx_id`,
		p.tenantID, txNo, p.pileID, p.connectorID, p.idTag, p.cardID, p.userID, p.deptID, p.vehicleID, p.bindMethod,
		p.startAt, p.meterStart, p.socStart, p.review, p.note).Scan(&id, &ocppID)
	if err != nil {
		return uuid.Nil, "", 0, err
	}
	return id, txNo, ocppID, tx.Commit(ctx)
}

type closeParams struct {
	id         uuid.UUID
	endAt      time.Time
	meterStop  float64
	kwh        float64
	reason     *string
	socEnd     *float64
	cc         CrossCheck
	review     string
	reviewNote *string
}

// close ends a charging transaction (status ended). Returns errNotCharging when it already ended.
func (s *store) close(ctx context.Context, p closeParams) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE charge_transactions SET status = 'ended', end_at = $2, meter_stop = $3, kwh = $4, stop_reason = $5,
		  bms_soc_end = $6, bms_kwh_est = $7, deviation_pct = $8, review_status = $9, review_note = $10
		WHERE id = $1 AND status = 'charging'`,
		p.id, p.endAt, p.meterStop, p.kwh, p.reason, p.socEnd, p.cc.BmsKwhEst, p.cc.DeviationPct, p.review, p.reviewNote)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotCharging
	}
	return nil
}

var errNotCharging = errors.New("transaction is not charging")

// settle stamps the billing result and marks the transaction settled.
func (s *store) settle(ctx context.Context, id uuid.UUID, r *BillingResult, review string, reviewedBy *uuid.UUID, note *string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE charge_transactions SET status = 'settled', unit_price = $2, cost = $3, attribution = $4, account_id = $5, account_txn_id = $6,
		  review_status = $7, reviewed_by = COALESCE($8, reviewed_by), reviewed_at = CASE WHEN $8::uuid IS NULL THEN reviewed_at ELSE now() END,
		  review_note = COALESCE($9, review_note)
		WHERE id = $1`, id, r.UnitPrice, r.Cost, r.Attribution, r.AccountID, r.AccountTxnID, review, reviewedBy, note)
	return err
}

// markPending keeps the transaction ended and queues it for review with a note.
func (s *store) markPending(ctx context.Context, id uuid.UUID, note string) error {
	_, err := s.db.Exec(ctx, `UPDATE charge_transactions SET review_status = 'pending', review_note = $2 WHERE id = $1 AND status IN ('ended','charging')`, id, note)
	return err
}

func (s *store) reject(ctx context.Context, id, by uuid.UUID, note *string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE charge_transactions SET status = 'ended', review_status = 'rejected', review_note = $3, reviewed_by = $2, reviewed_at = now(),
		  unit_price = NULL, cost = NULL, attribution = NULL, account_id = NULL, account_txn_id = NULL
		WHERE id = $1`, id, by, note)
	return err
}

type attributionParams struct {
	id         uuid.UUID
	userID     *uuid.UUID
	deptID     *uuid.UUID
	vehicleID  *uuid.UUID
	bindMethod string
	socStart   *float64
	socEnd     *float64
	cc         CrossCheck
}

// setAttribution rewrites who/which vehicle a transaction belongs to (review corrections).
func (s *store) setAttribution(ctx context.Context, p attributionParams) error {
	_, err := s.db.Exec(ctx, `
		UPDATE charge_transactions SET user_id = $2, dept_id = $3, vehicle_id = $4, bind_method = $5,
		  bms_soc_start = $6, bms_soc_end = $7, bms_kwh_est = $8, deviation_pct = $9
		WHERE id = $1`, p.id, p.userID, p.deptID, p.vehicleID, p.bindMethod, p.socStart, p.socEnd, p.cc.BmsKwhEst, p.cc.DeviationPct)
	return err
}

// ---- meter values

func (s *store) insertMeterValues(ctx context.Context, txID, pileID uuid.UUID, samples []MeterSample) error {
	if len(samples) == 0 {
		return nil
	}
	rows := make([][]any, len(samples))
	for i, m := range samples {
		var raw any
		if len(m.Raw) > 0 {
			raw = []byte(m.Raw)
		}
		rows[i] = []any{m.TS, txID, pileID, m.Wh, m.Voltage, m.Current, m.PowerKw, m.SOC, raw}
	}
	_, err := s.db.CopyFrom(ctx, pgx.Identifier{"charge_meter_values"},
		[]string{"ts", "tx_id", "pile_id", "wh", "voltage", "current", "power_kw", "soc", "raw"}, pgx.CopyFromRows(rows))
	return err
}

func (s *store) meterValues(ctx context.Context, txID uuid.UUID) ([]MeterValue, error) {
	var rows []MeterValue
	err := pgxscan.Select(ctx, s.db, &rows, `SELECT ts, wh, voltage, current, power_kw, soc FROM charge_meter_values WHERE tx_id = $1 ORDER BY ts`, txID)
	if rows == nil {
		rows = []MeterValue{}
	}
	return rows, err
}

func (s *store) lastMeter(ctx context.Context, txID uuid.UUID) (*MeterValue, error) {
	var m MeterValue
	err := pgxscan.Get(ctx, s.db, &m, `SELECT ts, wh, voltage, current, power_kw, soc FROM charge_meter_values WHERE tx_id = $1 ORDER BY ts DESC LIMIT 1`, txID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ---- aggregates

func (s *store) bucket(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (SummaryBucket, error) {
	var b SummaryBucket
	err := s.db.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(kwh), 0), COALESCE(sum(cost), 0) FROM charge_transactions
		WHERE tenant_id = $1 AND status <> 'cancelled' AND start_at >= $2 AND start_at < $3`, tenantID, from, to).
		Scan(&b.Sessions, &b.Kwh, &b.Cost)
	b.Kwh, b.Cost = round3(b.Kwh), round2(b.Cost)
	return b, err
}

func (s *store) counts(ctx context.Context, tenantID uuid.UUID) (ongoing, pending int, err error) {
	err = s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'charging'), count(*) FILTER (WHERE review_status = 'pending')
		FROM charge_transactions WHERE tenant_id = $1`, tenantID).Scan(&ongoing, &pending)
	return
}

func (s *store) byPile(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]PileSummary, error) {
	var rows []PileSummary
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT p.id AS pile_id, p.name AS pile_name, count(t.id) AS sessions, COALESCE(sum(t.kwh), 0) AS kwh, COALESCE(sum(t.cost), 0) AS cost
		FROM charge_piles p
		LEFT JOIN charge_transactions t ON t.pile_id = p.id AND t.status <> 'cancelled' AND t.start_at >= $2 AND t.start_at < $3
		WHERE p.tenant_id = $1 AND p.deleted_at IS NULL
		GROUP BY p.id, p.name ORDER BY p.pile_code`, tenantID, from, to)
	if rows == nil {
		rows = []PileSummary{}
	}
	for i := range rows {
		rows[i].Kwh, rows[i].Cost = round3(rows[i].Kwh), round2(rows[i].Cost)
	}
	return rows, err
}
