package bootstrap

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dictItem struct {
	Label, Value, Color string
}

type dictSeed struct {
	Code, Name, Description string
	Items                   []dictItem
}

// systemDicts are global (tenant_id NULL) system dictionaries. Only created when
// the type is missing; existing items are left to administrators.
var systemDicts = []dictSeed{
	{Code: "approval_purpose", Name: "用车事由", Description: "公务用车申请的事由分类", Items: []dictItem{
		{"公务出行", "official_trip", "blue"}, {"客户拜访", "client_visit", "green"}, {"政府对接", "gov_liaison", "purple"},
		{"会议出席", "meeting", "amber"}, {"物资采购", "procurement", "slate"}, {"接送人员", "shuttle", "cyan"}, {"其他", "other", "gray"},
	}},
	{Code: "trip_type", Name: "用车类型", Items: []dictItem{
		{"公务用车", "official", "blue"}, {"日常用车", "daily", "gray"},
	}},
	{Code: "approval_urgency", Name: "申请紧急程度", Items: []dictItem{
		{"普通", "normal", "gray"}, {"紧急", "urgent", "red"},
	}},
	{Code: "vehicle_status", Name: "车辆状态", Items: []dictItem{
		{"空闲", "idle", "green"}, {"在途", "in_use", "blue"}, {"充电中", "charging", "amber"},
		{"维保中", "maintenance", "red"}, {"停用", "disabled", "gray"},
	}},
	{Code: "vehicle_brand", Name: "车辆品牌", Items: []dictItem{
		{"比亚迪", "byd", ""}, {"特斯拉", "tesla", ""}, {"蔚来", "nio", ""}, {"小鹏", "xpeng", ""}, {"理想", "li", ""},
		{"广汽埃安", "aion", ""}, {"吉利", "geely", ""}, {"上汽", "saic", ""}, {"其他", "other", ""},
	}},
	{Code: "pile_type", Name: "充电桩类型", Items: []dictItem{
		{"快充", "fast", "amber"}, {"慢充", "slow", "green"},
	}},
	{Code: "trip_event_type", Name: "行程事件类型", Items: []dictItem{
		{"开始", "start", "blue"}, {"结束", "end", "gray"}, {"路线偏离", "deviation", "amber"},
		{"超速", "overspeed", "red"}, {"低电量", "low_soc", "amber"}, {"灯牌点亮", "sign_on", "blue"}, {"灯牌熄灭", "sign_off", "gray"},
	}},
}

func seedDicts(ctx context.Context, db *pgxpool.Pool) error {
	for _, d := range systemDicts {
		var id uuid.UUID
		err := db.QueryRow(ctx, `SELECT id FROM dict_types WHERE tenant_id IS NULL AND code = $1`, d.Code).Scan(&id)
		if err == nil {
			continue
		}
		if err := db.QueryRow(ctx, `
			INSERT INTO dict_types (tenant_id, code, name, description, is_system) VALUES (NULL, $1, $2, $3, true) RETURNING id`,
			d.Code, d.Name, d.Description).Scan(&id); err != nil {
			return err
		}
		for i, it := range d.Items {
			if _, err := db.Exec(ctx, `
				INSERT INTO dict_items (dict_type_id, label, value, sort, color) VALUES ($1,$2,$3,$4,NULLIF($5,''))
				ON CONFLICT (dict_type_id, value) DO NOTHING`, id, it.Label, it.Value, (i+1)*10, it.Color); err != nil {
				return err
			}
		}
	}
	return nil
}
