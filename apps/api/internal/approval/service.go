package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/notify"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app    *app.App
	store  *store
	notify *notify.Service
}

func NewService(a *app.App) *Service {
	return &Service{app: a, store: &store{db: a.DB}, notify: notify.NewService(a)}
}

const (
	maxDuration    = 7 * 24 * time.Hour
	pastTolerance  = 5 * time.Minute
	tenantAdminRol = "tenant_admin"
	timeLayout     = "01-02 15:04"
)

// ---- read

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, q ListQuery, pg pagination.Query) (pagination.Page[Approval], error) {
	if q.Scope == "" {
		q.Scope = "mine"
	}
	if q.Scope == "todo" && !actor.Has(PermApprove) {
		return pagination.Page[Approval]{}, httpx.Forbidden("没有权限：" + PermApprove)
	}
	if q.Scope == "all" && !actor.Has(PermManage) {
		return pagination.Page[Approval]{}, httpx.Forbidden("没有权限：" + PermManage)
	}
	f := listFilter{ListQuery: q, me: actor.UserID}
	var err error
	if f.from, err = parseTime(q.From, "from"); err != nil {
		return pagination.Page[Approval]{}, err
	}
	if f.to, err = parseTime(q.To, "to"); err != nil {
		return pagination.Page[Approval]{}, err
	}
	if q.DeptID != "" {
		id := uuid.MustParse(q.DeptID)
		f.deptID = &id
	}
	if _, err := s.store.expireStale(ctx, tenantID); err != nil {
		return pagination.Page[Approval]{}, err
	}
	rows, total, err := s.store.list(ctx, tenantID, f, pg)
	if err != nil {
		return pagination.Page[Approval]{}, err
	}
	items, err := s.assemble(ctx, rows, actor)
	if err != nil {
		return pagination.Page[Approval]{}, err
	}
	return pagination.NewPage(items, total, pg), nil
}

func parseTime(s, name string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, httpx.BadRequest(name + " 须为 RFC3339 时间")
	}
	return &t, nil
}

// Get returns the detail if the caller may see it (applicant, any approver, manage).
func (s *Service) Get(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID) (*Approval, error) {
	if _, err := s.store.expireStale(ctx, tenantID); err != nil {
		return nil, err
	}
	a, err := s.load(ctx, tenantID, actor, id)
	if err != nil {
		return nil, err
	}
	if !s.canView(a, actor) {
		return nil, httpx.Forbidden("无权查看该申请")
	}
	return a, nil
}

func (s *Service) load(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID) (*Approval, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("申请不存在")
	}
	items, err := s.assemble(ctx, []row{*r}, actor)
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (s *Service) canView(a *Approval, actor *auth.Principal) bool {
	if actor.Has(PermManage) || a.Applicant.ID == actor.UserID {
		return true
	}
	for _, st := range a.Steps {
		if st.Approver.ID == actor.UserID {
			return true
		}
	}
	return false
}

// assemble converts rows to API objects, attaching steps, trip summary and
// the caller-dependent can_approve / can_cancel flags.
func (s *Service) assemble(ctx context.Context, rows []row, actor *auth.Principal) ([]Approval, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	steps, err := s.store.steps(ctx, ids)
	if err != nil {
		return nil, err
	}
	trips, err := s.store.trips(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Approval, 0, len(rows))
	for _, r := range rows {
		a := toApproval(r)
		for _, st := range steps[r.ID] {
			a.Steps = append(a.Steps, Step{
				StepNo:   st.StepNo,
				Approver: UserBrief{ID: st.ApproverID, Name: st.Name, Username: st.Username, DeptName: st.DeptName, Phone: st.Phone},
				Action:   st.Action,
				Remark:   st.Remark,
				ActedAt:  st.ActedAt,
			})
		}
		a.Trip = trips[r.ID]
		pending := a.Status == StatusPendingL1 || a.Status == StatusPendingL2
		if actor != nil && pending {
			if actor.Has(PermManage) {
				a.CanApprove = true
			}
			for _, st := range a.Steps {
				if st.StepNo == a.CurrentStep && st.Action == ActionPending && st.Approver.ID == actor.UserID {
					a.CanApprove = true
				}
			}
		}
		if actor != nil && (pending || a.Status == StatusApproved) && (actor.Has(PermManage) || a.Applicant.ID == actor.UserID) {
			a.CanCancel = true
		}
		out = append(out, a)
	}
	return out, nil
}

func toApproval(r row) Approval {
	a := Approval{
		ID: r.ID, TenantID: r.TenantID, ApplyNo: r.ApplyNo,
		Applicant:     UserBrief{ID: r.ApplicantID, Name: r.ApplicantName, Username: r.ApplicantUsername, DeptName: r.ApplicantDeptName, Phone: r.ApplicantPhone},
		DeptID:        r.DeptID,
		DeptName:      r.DeptName,
		TripType:      r.TripType,
		PurposeCode:   r.PurposeCode,
		PurposeLabel:  r.PurposeLabel,
		PurposeDetail: r.PurposeDetail,
		PlannedStart:  r.PlannedStart,
		PlannedEnd:    r.PlannedEnd,
		Destination:   r.Destination,
		DestLng:       r.DestLng,
		DestLat:       r.DestLat,
		PlannedKm:     r.PlannedKm,
		Passengers:    []UserBrief{},
		Attachments:   []Attachment{},
		Urgency:       r.Urgency,
		Status:        r.Status,
		LevelRequired: r.LevelRequired,
		CurrentStep:   r.CurrentStep,
		Steps:         []Step{},
		RejectReason:  r.RejectReason,
		CancelReason:  r.CancelReason,
		ApprovedAt:    r.ApprovedAt,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	if len(r.PlannedRoute) > 0 {
		var pr PlannedRoute
		if json.Unmarshal(r.PlannedRoute, &pr) == nil {
			a.PlannedRoute = &pr
		}
	}
	if len(r.Passengers) > 0 {
		_ = json.Unmarshal(r.Passengers, &a.Passengers)
		if a.Passengers == nil {
			a.Passengers = []UserBrief{}
		}
	}
	if len(r.Attachments) > 0 {
		_ = json.Unmarshal(r.Attachments, &a.Attachments)
		if a.Attachments == nil {
			a.Attachments = []Attachment{}
		}
	}
	if r.VehicleID != nil && r.VehiclePlate != nil {
		a.Vehicle = &VehicleBrief{
			ID: *r.VehicleID, PlateNo: *r.VehiclePlate, Brand: r.VehicleBrand, Model: r.VehicleModel,
			Status: deref(r.VehicleStatus), SOC: r.VehicleSOC, RangeKm: r.VehicleRangeKm,
			HomeDeptID: r.VehicleHomeDeptID, HomeDeptName: r.VehicleHomeDeptName,
		}
	}
	return a
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---- precheck / create

// prepared is everything Create needs after a successful precheck.
type prepared struct {
	applicant  userRow
	passengers []UserBrief
	level      int
	approvers  []uuid.UUID
	result     PrecheckResult
}

// Precheck evaluates a request without persisting anything.
func (s *Service) Precheck(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, req CreateRequest) (*PrecheckResult, error) {
	p, err := s.prepare(ctx, tenantID, actor, req)
	if err != nil {
		return nil, err
	}
	return &p.result, nil
}

func (s *Service) prepare(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, req CreateRequest) (*prepared, error) {
	res := PrecheckResult{LevelRequired: 1, Level2Reasons: []string{}, Approvers: []UserBrief{}, Conflicts: []Conflict{}, Problems: []string{}}
	problem := func(msg string) { res.Problems = append(res.Problems, msg) }

	// applicant
	applicantID := actor.UserID
	if req.ApplicantID != nil && *req.ApplicantID != actor.UserID {
		if !actor.Has(PermManage) {
			return nil, httpx.Forbidden("代人发起需要 " + PermManage)
		}
		applicantID = *req.ApplicantID
	}
	applicant, err := s.store.user(ctx, tenantID, applicantID)
	if err != nil {
		return nil, err
	}
	if applicant == nil || applicant.Status != "active" {
		return nil, httpx.BadRequest("申请人不存在或已停用")
	}

	// time window
	now := time.Now()
	if !req.PlannedEnd.After(req.PlannedStart) {
		problem("结束时间必须晚于开始时间")
	}
	if req.PlannedStart.Before(now.Add(-pastTolerance)) {
		problem("开始时间不能早于当前时间")
	}
	if req.PlannedEnd.Sub(req.PlannedStart) > maxDuration {
		problem("用车时长不能超过 7 天")
	}

	// vehicle
	var vehicleHomeDept *uuid.UUID
	if req.VehicleID != nil {
		v, err := s.store.vehicle(ctx, tenantID, *req.VehicleID)
		if err != nil {
			return nil, err
		}
		switch {
		case v == nil || v.Deleted:
			problem("车辆不存在")
		case v.Status == "maintenance" || v.Status == "disabled":
			problem("车辆不可用（" + vehicleStatusLabel(v.Status) + "）")
		default:
			vehicleHomeDept = v.HomeDeptID
			if req.PlannedEnd.After(req.PlannedStart) {
				cs, err := s.store.conflicts(ctx, s.app.DB, tenantID, v.ID, req.PlannedStart, req.PlannedEnd, uuid.Nil)
				if err != nil {
					return nil, err
				}
				res.Conflicts = cs
				if len(cs) > 0 {
					problem("车辆时段冲突")
				}
			}
		}
	}

	// passengers
	var passengers []UserBrief
	if len(req.PassengerIDs) > 0 {
		users, err := s.store.users(ctx, tenantID, req.PassengerIDs)
		if err != nil {
			return nil, err
		}
		if len(users) != len(dedupe(req.PassengerIDs)) {
			problem("同行人包含不存在的用户")
		}
		for _, u := range users {
			passengers = append(passengers, u.UserBrief)
		}
	}
	if passengers == nil {
		passengers = []UserBrief{}
	}

	// level & approvers
	rules, err := s.loadRules(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	res.Level2Reasons = rules.Level2Reasons(Level2Input{
		PlannedKm: req.PlannedKm, Start: req.PlannedStart, End: req.PlannedEnd, TripType: req.TripType,
		VehicleHomeDept: vehicleHomeDept, ApplicantDept: applicant.DeptID,
	})
	l1, err := s.resolveLevel1(ctx, tenantID, *applicant, rules.FallbackApproverRole)
	if err != nil {
		return nil, err
	}
	approvers := []uuid.UUID{}
	if l1 == uuid.Nil {
		problem("无法确定审批人")
	} else {
		approvers = append(approvers, l1)
		if len(res.Level2Reasons) > 0 {
			l2, err := s.resolveLevel2(ctx, tenantID, rules, applicant.ID)
			if err != nil {
				return nil, err
			}
			switch {
			case l2 == uuid.Nil:
				problem("无法确定二级审批人")
			case l2 == l1:
				// same person on both levels → one step is enough
			default:
				approvers = append(approvers, l2)
			}
		}
	}
	res.LevelRequired = max(1, len(approvers))
	if len(approvers) > 0 {
		briefs, err := s.store.users(ctx, tenantID, approvers)
		if err != nil {
			return nil, err
		}
		byID := map[uuid.UUID]UserBrief{}
		for _, u := range briefs {
			byID[u.ID] = u.UserBrief
		}
		for _, id := range approvers {
			res.Approvers = append(res.Approvers, byID[id])
		}
	}
	res.OK = len(res.Problems) == 0
	return &prepared{applicant: *applicant, passengers: passengers, level: res.LevelRequired, approvers: approvers, result: res}, nil
}

func vehicleStatusLabel(s string) string {
	switch s {
	case "maintenance":
		return "维保中"
	case "disabled":
		return "已停用"
	case "in_use":
		return "使用中"
	}
	return s
}

// resolveLevel1 walks up the applicant's department chain for a leader that is
// not the applicant, then falls back to the role holders, then tenant admins.
func (s *Service) resolveLevel1(ctx context.Context, tenantID uuid.UUID, applicant userRow, fallbackRole string) (uuid.UUID, error) {
	deptID := applicant.DeptID
	for hops := 0; deptID != nil && hops < 32; hops++ {
		d, err := s.store.dept(ctx, tenantID, *deptID)
		if err != nil {
			return uuid.Nil, err
		}
		if d == nil {
			break
		}
		if d.LeaderUserID != nil && *d.LeaderUserID != applicant.ID {
			ok, err := s.store.userActive(ctx, tenantID, *d.LeaderUserID)
			if err != nil {
				return uuid.Nil, err
			}
			if ok {
				return *d.LeaderUserID, nil
			}
		}
		deptID = d.ParentID
	}
	exclude := []uuid.UUID{applicant.ID}
	for _, role := range []string{fallbackRole, tenantAdminRol} {
		if role == "" {
			continue
		}
		id, err := s.store.firstUserWithRole(ctx, tenantID, role, exclude)
		if err != nil || id != uuid.Nil {
			return id, err
		}
	}
	return uuid.Nil, nil
}

// resolveLevel2 prefers the configured approver, else a fallback-role holder.
func (s *Service) resolveLevel2(ctx context.Context, tenantID uuid.UUID, rules Rules, applicantID uuid.UUID) (uuid.UUID, error) {
	if rules.Level2ApproverID != nil {
		ok, err := s.store.userActive(ctx, tenantID, *rules.Level2ApproverID)
		if err != nil {
			return uuid.Nil, err
		}
		if ok {
			return *rules.Level2ApproverID, nil
		}
	}
	return s.store.firstUserWithRole(ctx, tenantID, rules.FallbackApproverRole, []uuid.UUID{applicantID})
}

// Create runs the precheck and persists the request; problems → 400 (409 when
// the only reason is a vehicle conflict).
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, req CreateRequest) (*Approval, error) {
	p, err := s.prepare(ctx, tenantID, actor, req)
	if err != nil {
		return nil, err
	}
	if !p.result.OK {
		msg := strings.Join(p.result.Problems, "；")
		if len(p.result.Conflicts) > 0 {
			return nil, httpx.Conflict(msg)
		}
		return nil, httpx.BadRequest(msg)
	}
	id, applyNo, err := s.store.insert(ctx, insertParams{
		tenantID: tenantID, applicant: p.applicant, req: req, passengers: p.passengers,
		level: p.level, approvers: p.approvers, createdBy: actor.UserID,
	})
	if err != nil {
		return nil, err
	}
	a, err := s.load(ctx, tenantID, actor, id)
	if err != nil {
		return nil, err
	}
	s.send(ctx, tenantID, []uuid.UUID{p.approvers[0]}, notify.TypeApprovalPending, "待审批：用车申请 "+applyNo,
		fmt.Sprintf("%s 申请 %s ~ %s 前往 %s", p.applicant.Name, fmtTime(req.PlannedStart), fmtTime(req.PlannedEnd), req.Destination), id)
	s.publish(tenantID, id, StatusPendingL1, applyNo)
	return a, nil
}

// ---- workflow actions

// Approve passes the current step; assigns / re-assigns the vehicle when asked.
func (s *Service) Approve(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, req ApproveRequest, ip string) (*Approval, error) {
	tx, err := s.app.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := lockApproval(ctx, tx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, httpx.NotFound("申请不存在")
	}
	next, err := NextStatus(a.Status, EvApprove, a.LevelRequired, a.CurrentStep)
	if err != nil {
		return nil, httpx.Conflict("当前状态不可审批：" + a.Status)
	}
	st, err := currentStep(ctx, tx, id, a.CurrentStep)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, httpx.Internal(fmt.Errorf("approval %s has no step %d", id, a.CurrentStep))
	}
	if st.ApproverID != actor.UserID && !actor.Has(PermManage) {
		return nil, httpx.Forbidden("仅当前步骤审批人可操作")
	}

	// vehicle assignment
	vehicleID := a.VehicleID
	switch {
	case a.VehicleID == nil && req.VehicleID == nil:
		return nil, httpx.BadRequest("申请未指定车辆，审批时必须指派 vehicle_id")
	case req.VehicleID != nil && (a.VehicleID == nil || *a.VehicleID != *req.VehicleID):
		if a.VehicleID != nil && !actor.Has(PermManage) {
			return nil, httpx.Forbidden("改派车辆需要 " + PermManage)
		}
		v, err := s.store.vehicle(ctx, tenantID, *req.VehicleID)
		if err != nil {
			return nil, err
		}
		if v == nil || v.Deleted {
			return nil, httpx.BadRequest("车辆不存在")
		}
		if v.Status == "maintenance" || v.Status == "disabled" {
			return nil, httpx.Conflict("车辆不可用（" + vehicleStatusLabel(v.Status) + "）")
		}
		cs, err := s.store.conflicts(ctx, tx, tenantID, v.ID, a.PlannedStart, a.PlannedEnd, id)
		if err != nil {
			return nil, err
		}
		if len(cs) > 0 {
			return nil, httpx.Conflict("车辆时段冲突：" + cs[0].ApplyNo)
		}
		vehicleID = &v.ID
	}

	if _, err := tx.Exec(ctx, `UPDATE approval_steps SET action = 'approved', remark = $3, acted_at = now(), ip = $4 WHERE approval_id = $1 AND step_no = $2`,
		id, a.CurrentStep, req.Remark, ip); err != nil {
		return nil, err
	}
	step := a.CurrentStep
	var approvedAt *time.Time
	if next == StatusPendingL2 {
		step++
	} else {
		now := time.Now()
		approvedAt = &now
	}
	if _, err := tx.Exec(ctx, `UPDATE approvals SET status = $2, current_step = $3, vehicle_id = $4, approved_at = COALESCE($5, approved_at) WHERE id = $1`,
		id, next, step, vehicleID, approvedAt); err != nil {
		return nil, err
	}
	var l2 *pendingStep
	if next == StatusPendingL2 {
		if l2, err = currentStep(ctx, tx, id, step); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if next == StatusPendingL2 {
		if l2 != nil {
			s.send(ctx, tenantID, []uuid.UUID{l2.ApproverID}, notify.TypeApprovalPending, "待审批（二级）：用车申请 "+a.ApplyNo, "一级审批已通过，请进行二级审批", id)
		}
		s.send(ctx, tenantID, []uuid.UUID{a.ApplicantID}, notify.TypeApprovalApproved, "一级审批已通过："+a.ApplyNo, "申请已通过一级审批，等待二级审批", id)
	} else {
		s.send(ctx, tenantID, []uuid.UUID{a.ApplicantID}, notify.TypeApprovalApproved, "申请已通过："+a.ApplyNo, "您的用车申请已批准", id)
	}
	s.publish(tenantID, id, next, a.ApplyNo)
	return s.load(ctx, tenantID, actor, id)
}

// Reject terminates the request at the current step.
func (s *Service) Reject(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, reason, ip string) (*Approval, error) {
	tx, err := s.app.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := lockApproval(ctx, tx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, httpx.NotFound("申请不存在")
	}
	if _, err := NextStatus(a.Status, EvReject, a.LevelRequired, a.CurrentStep); err != nil {
		return nil, httpx.Conflict("当前状态不可驳回：" + a.Status)
	}
	st, err := currentStep(ctx, tx, id, a.CurrentStep)
	if err != nil {
		return nil, err
	}
	if st != nil && st.ApproverID != actor.UserID && !actor.Has(PermManage) {
		return nil, httpx.Forbidden("仅当前步骤审批人可操作")
	}
	if _, err := tx.Exec(ctx, `UPDATE approval_steps SET action = 'rejected', remark = $3, acted_at = now(), ip = $4 WHERE approval_id = $1 AND step_no = $2`,
		id, a.CurrentStep, reason, ip); err != nil {
		return nil, err
	}
	if err := skipPending(ctx, tx, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE approvals SET status = 'rejected', reject_reason = $2 WHERE id = $1`, id, reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.send(ctx, tenantID, []uuid.UUID{a.ApplicantID}, notify.TypeApprovalRejected, "申请被驳回："+a.ApplyNo, "驳回原因："+reason, id)
	s.publish(tenantID, id, StatusRejected, a.ApplyNo)
	return s.load(ctx, tenantID, actor, id)
}

// Cancel withdraws a pending or approved (not yet started) request.
func (s *Service) Cancel(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, reason *string) (*Approval, error) {
	tx, err := s.app.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := lockApproval(ctx, tx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, httpx.NotFound("申请不存在")
	}
	if a.ApplicantID != actor.UserID && !actor.Has(PermManage) {
		return nil, httpx.Forbidden("仅申请人或管理员可撤销")
	}
	if _, err := NextStatus(a.Status, EvCancel, a.LevelRequired, a.CurrentStep); err != nil {
		return nil, httpx.Conflict("当前状态不可撤销：" + a.Status)
	}
	if ongoing, err := s.store.hasOngoingTrip(ctx, tx, id); err != nil {
		return nil, err
	} else if ongoing {
		return nil, httpx.Conflict("行程已开始，不能撤销")
	}
	var pendingApprover uuid.UUID
	if a.Status == StatusPendingL1 || a.Status == StatusPendingL2 {
		if st, err := currentStep(ctx, tx, id, a.CurrentStep); err != nil {
			return nil, err
		} else if st != nil {
			pendingApprover = st.ApproverID
		}
	}
	if err := skipPending(ctx, tx, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE approvals SET status = 'cancelled', cancel_reason = $2 WHERE id = $1`, id, reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if pendingApprover != uuid.Nil {
		s.send(ctx, tenantID, []uuid.UUID{pendingApprover}, notify.TypeApprovalCancelled, "申请已撤销："+a.ApplyNo, "申请人已撤销该用车申请，无需审批", id)
	}
	s.publish(tenantID, id, StatusCancelled, a.ApplyNo)
	return s.load(ctx, tenantID, actor, id)
}

func skipPending(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE approval_steps SET action = 'skipped' WHERE approval_id = $1 AND action = 'pending'`, id)
	return err
}

// ---- misc reads

func (s *Service) TodoCount(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal) (int64, error) {
	return s.store.todoCount(ctx, tenantID, actor.UserID)
}

func (s *Service) AvailableVehicles(ctx context.Context, tenantID uuid.UUID, q AvailableQuery) ([]VehicleBrief, error) {
	if !q.End.After(q.Start) {
		return nil, httpx.BadRequest("end 必须晚于 start")
	}
	exclude := uuid.Nil
	if q.ExcludeApprovalID != "" {
		exclude = uuid.MustParse(q.ExcludeApprovalID)
	}
	return s.store.availableVehicles(ctx, tenantID, q.Start, q.End, exclude)
}

// ---- rules

// loadRules returns the tenant rules or the defaults (approver brief attached).
func (s *Service) loadRules(ctx context.Context, tenantID uuid.UUID) (Rules, error) {
	r := DefaultRules()
	row, err := s.store.rules(ctx, tenantID)
	if err != nil {
		return r, err
	}
	if row != nil {
		r = Rules{
			Enabled: row.Enabled, Level2Km: row.Level2Km, Level2Night: row.Level2Night,
			NightStart: row.NightStart, NightEnd: row.NightEnd, Level2CrossDept: row.Level2CrossDept,
			Level2TripTypes: row.Level2TripTypes, Level2ApproverID: row.Level2ApproverID,
			FallbackApproverRole: row.FallbackApproverRole, OverdueAlertMinutes: row.OverdueAlertMinutes,
		}
		t := row.UpdatedAt
		r.UpdatedAt = &t
		if r.Level2TripTypes == nil {
			r.Level2TripTypes = []string{}
		}
	}
	if r.Level2ApproverID != nil {
		u, err := s.store.user(ctx, tenantID, *r.Level2ApproverID)
		if err != nil {
			return r, err
		}
		if u != nil {
			b := u.UserBrief
			r.Level2Approver = &b
		}
	}
	return r, nil
}

func (s *Service) GetRules(ctx context.Context, tenantID uuid.UUID) (*Rules, error) {
	r, err := s.loadRules(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SetRules applies the present fields onto the current rules and upserts.
func (s *Service) SetRules(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, u RulesUpdate) (*Rules, error) {
	if msg := ValidateRulesUpdate(&u); msg != "" {
		return nil, httpx.BadRequest(msg)
	}
	r, err := s.loadRules(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if u.Enabled != nil {
		r.Enabled = *u.Enabled
	}
	if u.Has("level2_km") {
		r.Level2Km = u.Level2Km
	}
	if u.Level2Night != nil {
		r.Level2Night = *u.Level2Night
	}
	if u.NightStart != nil {
		r.NightStart = strings.TrimSpace(*u.NightStart)
	}
	if u.NightEnd != nil {
		r.NightEnd = strings.TrimSpace(*u.NightEnd)
	}
	if u.Level2CrossDept != nil {
		r.Level2CrossDept = *u.Level2CrossDept
	}
	if u.Has("level2_trip_types") {
		r.Level2TripTypes = dedupeStrings(u.Level2TripTypes)
	}
	if u.Has("level2_approver_id") {
		if u.Level2ApproverID != nil {
			ok, err := s.store.userActive(ctx, tenantID, *u.Level2ApproverID)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, httpx.BadRequest("二级审批人不存在或已停用")
			}
		}
		r.Level2ApproverID = u.Level2ApproverID
	}
	if u.FallbackApproverRole != nil {
		r.FallbackApproverRole = strings.TrimSpace(*u.FallbackApproverRole)
	}
	if u.OverdueAlertMinutes != nil {
		r.OverdueAlertMinutes = *u.OverdueAlertMinutes
	}
	if err := s.store.upsertRules(ctx, tenantID, r, actor.UserID); err != nil {
		return nil, err
	}
	return s.GetRules(ctx, tenantID)
}

// ---- side effects

func (s *Service) send(ctx context.Context, tenantID uuid.UUID, users []uuid.UUID, typ, title, content string, approvalID uuid.UUID) {
	if err := s.notify.Send(ctx, tenantID, users, notify.Message{Type: typ, Title: title, Content: content, RefType: "approval", RefID: approvalID.String()}); err != nil {
		s.app.Log.Error().Err(err).Str("approval", approvalID.String()).Msg("approval notify failed")
	}
}

func (s *Service) publish(tenantID, id uuid.UUID, status, applyNo string) {
	s.app.Hub.Publish(tenantID, ws.Event{Type: ws.EvApprovalUpdated, Data: map[string]any{"approval_id": id, "status": status, "apply_no": applyNo}})
}

// PublishUpdated pushes approval.updated for status changes made by other modules.
func PublishUpdated(hub *ws.Hub, tenantID, id uuid.UUID, status, applyNo string) {
	hub.Publish(tenantID, ws.Event{Type: ws.EvApprovalUpdated, Data: map[string]any{"approval_id": id, "status": status, "apply_no": applyNo}})
}

func fmtTime(t time.Time) string { return t.In(Shanghai).Format(timeLayout) }

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	out := ids[:0:0]
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func dedupeStrings(ss []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, s := range ss {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
