package main

import (
	"context"
	"fmt"
	"path/filepath"
)

func (a *App) saveRouting(ctx context.Context, cfg Object, restart bool) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	return a.saveRoutingLocked(ctx, cfg, restart)
}

func (a *App) saveRoutingLocked(ctx context.Context, cfg Object, restart bool) error {
	apply := restart && str(a.store.snapshot()["routing_engine"]) != "xray"
	if apply && a.routing == nil {
		return fmt.Errorf("OptiRout process management is disabled")
	}
	previous, e := a.store.loadOptirout()
	if e != nil {
		return e
	}
	if e = a.store.saveOptirout(cfg); e != nil {
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
