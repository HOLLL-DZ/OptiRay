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
	extra, e := a.routingPowerTransition(before, next, enabled)
	if e != nil {
		return e
	}
	return a.commitState(ctx, before, next, extra)
}

func (a *App) routingPowerTransition(before, next Object, enabled bool) (map[string][]byte, error) {
	cfg, e := a.store.loadOptirout()
	if e != nil {
		return nil, e
	}
	if !enabled {
		next["optiray_resume_optirout"] = configEnabled(cfg)
		cfg["enabled"] = false
	} else {
		wanted, known := before["optiray_resume_optirout"]
		if known {
			cfg["enabled"] = boolean(wanted)
		} else if !optirayEnabled(before) {
			// Older versions cleared this flag without retaining the original preference.
			cfg["enabled"] = true
		}
		delete(next, "optiray_resume_optirout")
	}
	raw, e := json.Marshal(cfg)
	if e != nil {
		return nil, e
	}
	return map[string][]byte{"routing/optirout.json": raw}, nil
}
