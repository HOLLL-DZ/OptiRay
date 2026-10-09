package main

import (
	"context"
	"fmt"
	"time"
)

// applyRuntime returns an undo for router changes; core/routing rollback uses the old document.
func (a *App) applyRuntime(ctx context.Context, d Object) (func() error, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if a.routing != nil {
		if !a.routingWanted(d) {
			if e := a.routing.Stop(); e != nil {
				return nil, e
			}
		} else {
			if _, e := a.store.loadOptirout(); e != nil {
				return nil, e
			}
			if e := a.routing.Restart(ctx); e != nil {
				return nil, e
			}
		}
	}
	server, _ := findServer(d, str(d["selected"]), "")
	if server != nil && optirayEnabled(d) {
		if a.core == nil {
			return nil, fmt.Errorf("active core management is disabled")
		}
		if e := a.core.ApplyServer(server, d, integer(d["proxy_port"])); e != nil {
			return nil, e
		}
	} else if a.core != nil {
		a.core.Stop()
	}
	if a.keenetic != nil && optirayEnabled(d) {
		return a.keenetic.Apply(ctx, str(d["proxy_name"]), str(d["proxy_host"]), integer(d["proxy_port"]))
	}
	return nil, nil
}
func (a *App) rollbackRuntime(d Object, undo func() error) error {
	if undo != nil {
		if e := undo(); e != nil {
			return fmt.Errorf("proxy rollback: %w", e)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// The explicit undo has restored the proxy; do not adopt it again.
	if a.routing != nil {
		if !a.routingWanted(d) {
			if e := a.routing.Stop(); e != nil {
				return e
			}
		} else {
			if e := a.routing.Restart(ctx); e != nil {
				return e
			}
		}
	}
	server, _ := findServer(d, str(d["selected"]), "")
	if a.core != nil {
		if server == nil || !optirayEnabled(d) {
			a.core.Stop()
		} else {
			return a.core.ApplyServer(server, d, integer(d["proxy_port"]))
		}
	}
	return nil
}
