package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

func optirayEnabled(d Object) bool { v, ok := d["optiray_enabled"]; return !ok || boolean(v) }
func configEnabled(d Object) bool  { v, ok := d["enabled"]; return !ok || boolean(v) }
func (a *App) routingWanted(d Object) bool {
	if str(d["routing_engine"]) == "xray" {
		return false
	}
	cfg, e := readObject(filepath.Join(a.store.dir, "routing", "optirout.json"))
	return e != nil || configEnabled(cfg)
}
func (a *App) setOptirayEnabled(ctx context.Context, enabled bool) error {
	if !a.connectMu.TryLock() {
		return fmt.Errorf("Операция уже выполняется. Подождите")
	}
	defer a.connectMu.Unlock()
	if time.Since(a.lastPowerSwitch) < 3*time.Second {
		return fmt.Errorf("Подождите 3 секунды перед повторным переключением")
	}
	before := a.store.snapshot()
	if optirayEnabled(before) == enabled {
		return nil
	}
	next := clone(before)
	next["optiray_enabled"] = enabled
	a.lastPowerSwitch = time.Now()
	defer func() { a.lastPowerSwitch = time.Now() }()
	var extra map[string][]byte
	if !enabled {
		cfg, e := a.store.loadOptirout()
		if e != nil {
			return e
		}
		cfg["enabled"] = false
		raw, e := json.Marshal(cfg)
		if e != nil {
			return e
		}
		extra = map[string][]byte{"routing/optirout.json": raw}
	}
	return a.commitState(ctx, before, next, extra)
}
