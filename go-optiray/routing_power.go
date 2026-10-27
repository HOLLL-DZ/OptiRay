package main

import (
	"context"
	"fmt"
)

// Power changes use saved rules only: never commit an editor's unsaved draft.
func (a *App) setRoutingPower(ctx context.Context, engine string, enabled bool) error {
	if engine != "optirout" && engine != "xray" {
		return fmt.Errorf("invalid routing engine")
	}
	if !a.connectMu.TryLock() {
		return fmt.Errorf("Операция уже выполняется. Подождите")
	}
	defer a.connectMu.Unlock()
	before := a.store.snapshot()
	if str(before["routing_engine"]) != engine {
		return fmt.Errorf("Движок маршрутизации изменился. Откройте редактор заново")
	}
	if engine == "optirout" {
		cfg, err := a.store.loadOptirout()
		if err != nil {
			return err
		}
		if configEnabled(cfg) == enabled {
			return nil
		}
		cfg["enabled"] = enabled
		return a.saveRoutingLocked(ctx, cfg, true)
	}
	if boolean(obj(before["routing"])["enabled"]) == enabled {
		return nil
	}
	next := clone(before)
	routing := obj(next["routing"])
	routing["enabled"] = enabled
	next["routing"] = routing
	return a.commitState(ctx, before, next, nil)
}
