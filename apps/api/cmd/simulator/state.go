package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// State is persisted between runs so assets (and device api keys, which the
// API only returns once) are reused instead of re-created.
type State struct {
	Vehicles []*VehicleAsset `json:"vehicles"`
	Drivers  []*DriverAsset  `json:"drivers"`
	Piles    []*PileAsset    `json:"piles"`
	SavedAt  time.Time       `json:"saved_at"`
}

type VehicleAsset struct {
	PlateNo    string  `json:"plate_no"`
	ID         string  `json:"id"`
	DeviceID   string  `json:"device_id"`
	Serial     string  `json:"serial_no"`
	APIKey     string  `json:"api_key"`
	Lng        float64 `json:"lng"`
	Lat        float64 `json:"lat"`
	SOC        float64 `json:"soc"`
	OdometerKm float64 `json:"odometer_km"`
}

type DriverAsset struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	ID       string `json:"id"`
	CardUID  string `json:"card_uid"`
}

type PileAsset struct {
	Code string  `json:"pile_code"`
	Name string  `json:"name"`
	ID   string  `json:"id"`
	Lng  float64 `json:"lng"`
	Lat  float64 `json:"lat"`
}

// loadState reads the file; a missing file yields an empty state.
func loadState(path string) (*State, error) {
	st := &State{}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, err
	}
	return st, nil
}

// save writes atomically (temp file + rename).
func (s *State) save(path string) error {
	s.SavedAt = time.Now()
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *State) vehicle(plate string) *VehicleAsset {
	for _, v := range s.Vehicles {
		if v.PlateNo == plate {
			return v
		}
	}
	return nil
}

func (s *State) driver(username string) *DriverAsset {
	for _, d := range s.Drivers {
		if d.Username == username {
			return d
		}
	}
	return nil
}

func (s *State) pile(code string) *PileAsset {
	for _, p := range s.Piles {
		if p.Code == code {
			return p
		}
	}
	return nil
}
