package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
)

// provisioner creates or reuses the demo assets as the tenant admin.
type provisioner struct {
	cfg *Config
	c   *client
	st  *State
	log *slog.Logger
	rng *rand.Rand

	purposes []string
}

type page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// listAll walks a paginated list endpoint.
func listAll[T any](ctx context.Context, c *client, path string) ([]T, error) {
	var all []T
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for p := 1; p <= 50; p++ {
		var pg page[T]
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s%spage=%d&pageSize=200", path, sep, p), nil, &pg); err != nil {
			return nil, err
		}
		all = append(all, pg.Items...)
		if len(pg.Items) < 200 {
			break
		}
	}
	return all, nil
}

type apiVehicle struct {
	ID      string `json:"id"`
	PlateNo string `json:"plate_no"`
}

type apiDevice struct {
	ID        string  `json:"id"`
	SerialNo  string  `json:"serial_no"`
	VehicleID *string `json:"vehicle_id"`
	APIKey    string  `json:"api_key"`
}

type apiUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type apiRole struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type apiCard struct {
	ID      string  `json:"id"`
	CardUID string  `json:"card_uid"`
	UserID  *string `json:"user_id"`
	Status  string  `json:"status"`
}

type apiPile struct {
	ID       string   `json:"id"`
	PileCode string   `json:"pile_code"`
	Lng      *float64 `json:"lng"`
	Lat      *float64 `json:"lat"`
}

type apiDictItem struct {
	Value string `json:"value"`
}

// ensureAll provisions everything. Vehicles and devices are mandatory; the
// rest degrades gracefully (no drivers → telemetry-only, no piles → no charging).
func (p *provisioner) ensureAll(ctx context.Context) error {
	if err := p.ensureVehicles(ctx); err != nil {
		return fmt.Errorf("vehicles: %w", err)
	}
	if err := p.ensureDevices(ctx); err != nil {
		return fmt.Errorf("devices: %w", err)
	}
	if err := p.ensureDrivers(ctx); err != nil {
		p.log.Warn("drivers unavailable, running telemetry-only", "err", err)
		p.cfg.TelemetryOnly = true
	} else if err := p.ensureCards(ctx); err != nil {
		p.log.Warn("nfc cards unavailable, running telemetry-only", "err", err)
		p.cfg.TelemetryOnly = true
	}
	if err := p.ensurePiles(ctx); err != nil {
		p.log.Warn("charge piles unavailable, charging disabled", "err", err)
	}
	p.purposes = []string{"official_trip"}
	var items []apiDictItem
	if err := p.c.do(ctx, http.MethodGet, "/system/dicts/approval_purpose", nil, &items); err != nil {
		p.log.Warn("approval_purpose dict unavailable, using default", "err", err)
	} else if len(items) > 0 {
		p.purposes = p.purposes[:0]
		for _, it := range items {
			p.purposes = append(p.purposes, it.Value)
		}
	}
	return nil
}

func (p *provisioner) ensureVehicles(ctx context.Context) error {
	existing, err := listAll[apiVehicle](ctx, p.c, "/assets/vehicles")
	if err != nil {
		return err
	}
	byPlate := map[string]apiVehicle{}
	for _, v := range existing {
		byPlate[v.PlateNo] = v
	}
	kept := make([]*VehicleAsset, 0, p.cfg.Vehicles)
	for i := 1; i <= p.cfg.Vehicles; i++ {
		plate := fmt.Sprintf("鲁A·S%04d", i)
		va := p.st.vehicle(plate)
		if va == nil {
			va = &VehicleAsset{PlateNo: plate}
		}
		if va.Lng == 0 || va.Lat == 0 {
			pos := randomPointWithin(p.rng, p.cfg.Center, p.cfg.RadiusKm*0.5)
			va.Lng, va.Lat = round(pos.Lng, 6), round(pos.Lat, 6)
		}
		if va.SOC <= 0 {
			va.SOC = round(55+p.rng.Float64()*40, 1)
		}
		if va.OdometerKm <= 0 {
			va.OdometerKm = float64(1000 + p.rng.Intn(30000))
		}
		if v, ok := byPlate[plate]; ok {
			va.ID = v.ID
		} else {
			var v apiVehicle
			body := map[string]any{
				"plate_no": plate, "brand": "比亚迪", "model": "e6", "color": "白色", "seat_count": 5,
				"battery_kwh": 60, "range_km_full": 400, "odometer_km": va.OdometerKm, "remark": "模拟器创建",
			}
			if err := p.c.do(ctx, http.MethodPost, "/assets/vehicles", body, &v); err != nil {
				return err
			}
			va.ID = v.ID
			p.log.Info("vehicle created", "plate", plate, "id", v.ID)
		}
		kept = append(kept, va)
	}
	p.st.Vehicles = kept
	return nil
}

func (p *provisioner) ensureDevices(ctx context.Context) error {
	existing, err := listAll[apiDevice](ctx, p.c, "/assets/devices")
	if err != nil {
		return err
	}
	bySerial := map[string]apiDevice{}
	for _, d := range existing {
		bySerial[d.SerialNo] = d
	}
	for i, va := range p.st.Vehicles {
		serial := fmt.Sprintf("SIM-%04d", i+1)
		va.Serial = serial
		d, ok := bySerial[serial]
		if !ok {
			var out apiDevice
			body := map[string]any{"serial_no": serial, "vehicle_id": va.ID, "model": "VIG-100E", "firmware": "sim-1.0.0", "remark": "模拟网关"}
			if err := p.c.do(ctx, http.MethodPost, "/assets/devices", body, &out); err != nil {
				return err
			}
			va.DeviceID, va.APIKey = out.ID, out.APIKey
			p.log.Info("device created", "serial", serial, "vehicle", va.PlateNo)
			continue
		}
		va.DeviceID = d.ID
		if d.VehicleID == nil || *d.VehicleID != va.ID {
			if d.VehicleID != nil {
				if err := p.c.do(ctx, http.MethodPost, "/assets/devices/"+d.ID+"/unbind", nil, nil); err != nil {
					p.log.Warn("unbind stale device binding failed", "serial", serial, "err", err)
				}
			}
			if err := p.c.do(ctx, http.MethodPost, "/assets/devices/"+d.ID+"/bind", map[string]string{"vehicle_id": va.ID}, nil); err != nil {
				return fmt.Errorf("bind %s: %w", serial, err)
			}
			p.log.Info("device bound", "serial", serial, "vehicle", va.PlateNo)
		}
		if va.APIKey == "" {
			var out apiDevice
			if err := p.c.do(ctx, http.MethodPost, "/assets/devices/"+d.ID+"/rotate-key", nil, &out); err != nil {
				return fmt.Errorf("rotate key %s: %w", serial, err)
			}
			va.APIKey = out.APIKey
			p.log.Info("device key rotated (no key in state file)", "serial", serial)
		}
	}
	return nil
}

var driverNames = []string{"张三", "李四", "王五"}

func (p *provisioner) ensureDrivers(ctx context.Context) error {
	var roles []apiRole
	employeeRole := ""
	if err := p.c.do(ctx, http.MethodGet, "/system/roles/options", nil, &roles); err != nil {
		p.log.Warn("role options unavailable, creating drivers without role", "err", err)
	}
	for _, r := range roles {
		if r.Code == "employee" {
			employeeRole = r.ID
		}
	}
	existing, err := listAll[apiUser](ctx, p.c, "/system/users?keyword=sim_driver_")
	if err != nil {
		return err
	}
	byName := map[string]apiUser{}
	for _, u := range existing {
		byName[u.Username] = u
	}
	kept := make([]*DriverAsset, 0, len(driverNames))
	for i, name := range driverNames {
		username := fmt.Sprintf("sim_driver_%d", i+1)
		da := p.st.driver(username)
		if da == nil {
			da = &DriverAsset{Username: username}
		}
		da.Name = name
		da.CardUID = fmt.Sprintf("A1B2C3D%d", i)
		if u, ok := byName[username]; ok {
			da.ID = u.ID
		} else {
			body := map[string]any{"username": username, "name": name, "password": "Sim@123456", "employee_no": fmt.Sprintf("SIM%03d", i+1)}
			if employeeRole != "" {
				body["role_ids"] = []string{employeeRole}
			}
			var u apiUser
			if err := p.c.do(ctx, http.MethodPost, "/system/users", body, &u); err != nil {
				return err
			}
			da.ID = u.ID
			p.log.Info("driver created", "username", username, "name", name)
		}
		kept = append(kept, da)
	}
	p.st.Drivers = kept
	return nil
}

func (p *provisioner) ensureCards(ctx context.Context) error {
	existing, err := listAll[apiCard](ctx, p.c, "/assets/nfc-cards")
	if err != nil {
		return err
	}
	byUID := map[string]apiCard{}
	for _, c := range existing {
		byUID[strings.ToUpper(c.CardUID)] = c
	}
	for _, da := range p.st.Drivers {
		c, ok := byUID[da.CardUID]
		if !ok {
			body := map[string]any{"card_uid": da.CardUID, "user_id": da.ID, "remark": "模拟驾驶员 " + da.Name}
			if err := p.c.do(ctx, http.MethodPost, "/assets/nfc-cards", body, nil); err != nil {
				return err
			}
			p.log.Info("nfc card issued", "uid", da.CardUID, "driver", da.Name)
			continue
		}
		if c.UserID == nil || *c.UserID != da.ID {
			if err := p.c.do(ctx, http.MethodPost, "/assets/nfc-cards/"+c.ID+"/bind", map[string]string{"user_id": da.ID}, nil); err != nil {
				return err
			}
			p.log.Info("nfc card re-bound", "uid", da.CardUID, "driver", da.Name)
		}
		if c.Status != "active" {
			if err := p.c.do(ctx, http.MethodPut, "/assets/nfc-cards/"+c.ID, map[string]string{"status": "active"}, nil); err != nil {
				p.log.Warn("re-activate card failed", "uid", da.CardUID, "err", err)
			}
		}
	}
	return nil
}

var pileDefs = []struct {
	code, name string
	dLng, dLat float64
}{
	{"SIM-PILE-01", "模拟快充站 A（高新区）", 0.028, 0.012},
	{"SIM-PILE-02", "模拟快充站 B（市中）", -0.024, -0.009},
}

func (p *provisioner) ensurePiles(ctx context.Context) error {
	existing, err := listAll[apiPile](ctx, p.c, "/assets/charge-piles")
	if err != nil {
		return err
	}
	byCode := map[string]apiPile{}
	for _, x := range existing {
		byCode[x.PileCode] = x
	}
	kept := make([]*PileAsset, 0, len(pileDefs))
	for _, def := range pileDefs {
		pa := p.st.pile(def.code)
		if pa == nil {
			pa = &PileAsset{Code: def.code}
		}
		pa.Name = def.name
		pa.Lng, pa.Lat = round(p.cfg.Center.Lng+def.dLng, 6), round(p.cfg.Center.Lat+def.dLat, 6)
		if x, ok := byCode[def.code]; ok {
			pa.ID = x.ID
			if x.Lng != nil && x.Lat != nil {
				pa.Lng, pa.Lat = *x.Lng, *x.Lat
			}
		} else {
			body := map[string]any{
				"pile_code": def.code, "name": def.name, "type": "fast", "power_kw": 60, "connector_count": 2,
				"vendor": "模拟", "location": "济南市（模拟站点）", "lng": pa.Lng, "lat": pa.Lat, "remark": "模拟器创建",
			}
			var x apiPile
			if err := p.c.do(ctx, http.MethodPost, "/assets/charge-piles", body, &x); err != nil {
				return err
			}
			pa.ID = x.ID
			p.log.Info("charge pile created", "code", def.code)
		}
		kept = append(kept, pa)
	}
	p.st.Piles = kept
	return nil
}
