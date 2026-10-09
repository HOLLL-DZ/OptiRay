package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func defaultData() (Object, error) {
	b, e := assets.ReadFile("assets/defaults.json")
	if e != nil {
		return nil, e
	}
	d := Object{}
	e = json.Unmarshal(b, &d)
	return d, e
}
func defaultOptirout() (Object, error) {
	b, e := assets.ReadFile("assets/optirout-default.json")
	if e != nil {
		return nil, e
	}
	d := Object{}
	if e = json.Unmarshal(b, &d); e != nil {
		return nil, e
	}
	return sanitizeOptirout(d)
}

// Caller serializes connection mutations. Disk and runtime are restored on failure.
func (a *App) commitState(ctx context.Context, before, next Object, extra map[string][]byte) error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	oldJSON, _ := json.Marshal(before)
	currentJSON, _ := json.Marshal(a.store.data)
	if string(oldJSON) != string(currentJSON) {
		return fmt.Errorf("settings changed concurrently; retry")
	}
	oldOpt, e := a.store.loadOptiroutLocked()
	if e != nil {
		return e
	}
	if e = a.store.persistExtra(next, extra); e != nil {
		return e
	}
	// loadOptirout is otherwise locked by applyRuntime, so runtime uses files directly here.
	if a.routing != nil {
		if !a.routingWanted(next) {
			e = a.routing.Stop()
		} else {
			e = a.routing.Restart(ctx)
		}
	}
	if e == nil {
		srv, _ := findServer(next, str(next["selected"]), "")
		if srv != nil && optirayEnabled(next) {
			if a.core == nil {
				e = fmt.Errorf("active core management is disabled")
			} else {
				e = a.core.ApplyServer(srv, next, integer(next["proxy_port"]))
			}
		} else if a.core != nil {
			a.core.Stop()
		}
	}
	var undo func() error
	if e == nil && a.keenetic != nil && optirayEnabled(next) {
		undo, e = a.keenetic.Apply(ctx, str(next["proxy_name"]), str(next["proxy_host"]), integer(next["proxy_port"]))
	}
	if e != nil {
		oldBytes, _ := json.Marshal(oldOpt)
		rollback := a.store.persistExtra(before, map[string][]byte{"routing/optirout.json": oldBytes})
		if rollback != nil {
			return fmt.Errorf("apply failed: %v; settings rollback failed: %v", e, rollback)
		}
		if rollback = a.rollbackRuntime(before, undo); rollback != nil {
			return fmt.Errorf("apply failed: %v; runtime rollback failed: %v", e, rollback)
		}
		return e
	}
	a.store.data = next
	return nil
}
func (a *App) restoreState(ctx context.Context, payload Object) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	before := a.store.snapshot()
	next, e := defaultData()
	if e != nil {
		return e
	}
	if _, ok := payload["subscriptions"].([]any); !ok {
		return fmt.Errorf("subscriptions required")
	}
	ids := map[int]bool{}
	for _, v := range list(payload["subscriptions"]) {
		s := obj(v)
		id := integer(s["id"])
		if id < 1 || ids[id] {
			return fmt.Errorf("invalid or duplicate subscription ID")
		}
		ids[id] = true
		if _, ok := s["servers"].([]any); !ok {
			return fmt.Errorf("invalid server list")
		}
	}
	for k, v := range clone(payload) {
		next[k] = v
	}
	if payload["routing"] == nil {
		next["routing"] = before["routing"]
	}
	delete(next, "optirout_config")
	if e = validateRouting(obj(next["routing"])); e != nil {
		return e
	}
	if !validProxyParams(str(next["proxy_name"]), str(next["proxy_host"]), integer(next["proxy_port"])) || strings.TrimSpace(str(next["hwid"])) == "" {
		return fmt.Errorf("invalid restored settings")
	}
	if str(next["routing_engine"]) != "xray" && str(next["routing_engine"]) != "optirout" {
		return fmt.Errorf("invalid routing engine")
	}
	next["selected"] = ""
	next["schema_version"] = 3
	extra := map[string][]byte{}
	if payload["optirout_config"] != nil {
		cfg, e := sanitizeOptirout(obj(payload["optirout_config"]))
		if e != nil {
			return e
		}
		b, e := json.Marshal(cfg)
		if e != nil {
			return e
		}
		extra["routing/optirout.json"] = b
	}
	return a.commitState(ctx, before, next, extra)
}
func (a *App) routingMode(ctx context.Context, engine string, routing Object) error {
	if engine != "xray" && engine != "optirout" {
		return fmt.Errorf("invalid routing engine")
	}
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	before := a.store.snapshot()
	next := clone(before)
	next["routing_engine"] = engine
	if routing != nil {
		if e := validateRouting(routing); e != nil {
			return e
		}
		for k, v := range routing {
			obj(next["routing"])[k] = v
		}
	}
	return a.commitState(ctx, before, next, nil)
}
func (a *App) resetState(ctx context.Context) error {
	d, e := defaultData()
	if e != nil {
		return e
	}
	cfg, e := defaultOptirout()
	if e != nil {
		return e
	}
	current := a.store.snapshot()
	d["hwid"] = current["hwid"]
	d["proxy_name"] = current["proxy_name"]
	d["proxy_host"] = current["proxy_host"]
	d["proxy_port"] = current["proxy_port"]
	d["optirout_config"] = cfg
	return a.restoreState(ctx, d)
}
func (a *App) readRoutingBackup(name string) (Object, error) {
	if name == "" || strings.ContainsAny(name, "/\\:") || !strings.HasSuffix(name, ".json") {
		return nil, fmt.Errorf("invalid backup name")
	}
	return readObject(filepath.Join(a.store.dir, "routing", "backups", "optirout", name))
}
