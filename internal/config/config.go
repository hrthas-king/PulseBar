// Package config 管理用户界面与刷新设置（%LOCALAPPDATA%\PulseBar\config.json）。不持久化公网 IP 或流量历史。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	IntervalSeconds int    `json:"interval_seconds"`
	ShowNetDetail   bool   `json:"show_net_detail"`
	Locked          bool   `json:"locked"`
	UpIsRed         bool   `json:"up_is_red"`
	Placement       string `json:"placement"`
	X               *int   `json:"x,omitempty"`
	Y               *int   `json:"y,omitempty"`
}

func ValidInterval(seconds int) bool {
	return seconds == 1 || seconds == 2 || seconds == 5
}

func Default() Config {
	return Config{
		IntervalSeconds: 2,
		ShowNetDetail:   false,
		Locked:          false,
		UpIsRed:         false,
		Placement:       "left",
	}
}

func Path() string {
	if override := os.Getenv("PULSEBAR_CONFIG"); override != "" {
		return override
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "PulseBar", "config.json")
}

func Load() Config {
	return LoadFrom(Path())
}

func LoadFrom(path string) Config {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var saved Config
	if json.Unmarshal(data, &saved) != nil {
		return c
	}
	if ValidInterval(saved.IntervalSeconds) {
		c.IntervalSeconds = saved.IntervalSeconds
	}
	c.ShowNetDetail = saved.ShowNetDetail
	c.Locked = saved.Locked
	c.UpIsRed = saved.UpIsRed
	if saved.Placement == "manual" && saved.X != nil && saved.Y != nil &&
		abs(*saved.X) < 100000 && abs(*saved.Y) < 100000 {
		x, y := *saved.X, *saved.Y
		c.Placement = "manual"
		c.X = &x
		c.Y = &y
	} else {
		c.Placement = "left"
		c.X = nil
		c.Y = nil
	}
	return c
}

func Save(c Config) error {
	return SaveTo(Path(), c)
}

func SaveTo(path string, c Config) error {
	if !ValidInterval(c.IntervalSeconds) {
		return fmt.Errorf("刷新间隔只能为 1、2、5 秒")
	}
	if c.Placement != "manual" || c.X == nil || c.Y == nil {
		c.Placement = "left"
		c.X = nil
		c.Y = nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
