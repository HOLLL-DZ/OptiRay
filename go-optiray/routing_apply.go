package main

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
)

func (a *App) saveRouting(ctx context.Context, cfg Object, restart bool) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	return a.saveRoutingLocked(ctx, cfg, restart)
}

func (a *App) saveRoutingLocked(ctx context.Context, cfg Object, restart bool) error {
	previous, e := a.store.loadOptirout()
	if e != nil {
		return e
	}
	clean, e := sanitizeOptirout(cfg)
	if e != nil {
		return e
	}
	previousClean, e := sanitizeOptirout(previous)
	if e != nil {
		return e
	}
	if reflect.DeepEqual(previousClean, clean) {
		return nil
	}
	apply := restart && optiroutRuntimeChanged(previousClean, clean) && str(a.store.snapshot()["routing_engine"]) != "xray"
	if apply && a.routing == nil {
		return fmt.Errorf("OptiRout process management is disabled")
	}
	if e = a.store.saveOptirout(clean); e != nil {
		return e
	}
	if !apply {
		return nil
	}
	if a.routingWanted(a.store.snapshot()) {
		e = a.routing.Restart(ctx)
	} else {
		e = a.routing.Stop()
	}
	if e != nil {
		a.store.mu.Lock()
		restore := writeObject(filepath.Join(a.store.dir, "routing", "optirout.json"), previous)
		a.store.mu.Unlock()
		if restore != nil {
			return fmt.Errorf("routing restart failed; configuration rollback failed")
		}
		var rollback error
		if a.routingWanted(a.store.snapshot()) {
			rollback = a.routing.Restart(context.Background())
		} else {
			rollback = a.routing.Stop()
		}
		if rollback != nil {
			return fmt.Errorf("routing restart failed: %v; restored engine restart failed: %v", e, rollback)
		}
		return fmt.Errorf("routing restart failed; previous engine restored: %w", e)
	}
	return nil
}

func optiroutRuntimeChanged(before, after Object) bool {
	before, after = clone(before), clone(after)
	for _, cfg := range []Object{before, after} {
		cfg["enabled"] = configEnabled(cfg)
		for _, group := range list(cfg["groups"]) {
			delete(obj(group), "color")
			delete(obj(group), "geo_notice")
		}
	}
	return !reflect.DeepEqual(before, after)
}
