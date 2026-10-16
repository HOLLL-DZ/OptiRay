package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPowerStatePersistsAndBlocksRecovery(t *testing.T) {
	dir := t.TempDir()
	s, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	// A selected server must remain remembered while its core is disabled.
	if e = s.change(func(d Object) error {
		d["selected"] = "remembered"
		d["subscriptions"] = []any{Object{"id": 1, "servers": []any{Object{"sid": "remembered", "name": "Test", "outbound": Object{"protocol": "freedom"}}}}}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	a := &App{store: s, core: &CoreManager{bin: "missing", dir: dir}}
	if e = a.setOptirayEnabled(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	again, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	if optirayEnabled(again.snapshot()) || str(again.snapshot()["selected"]) != "remembered" {
		t.Fatal("disabled state or selection lost")
	}
	if e = a.setOptirayEnabled(context.Background(), true); e == nil {
		t.Fatal("rapid toggling allowed")
	}
	if a.routingWanted(again.snapshot()) {
		t.Fatal("routing starts with disabled core")
	}
	if e = a.core.ApplyServer(Object{}, again.snapshot(), 3080); e != nil {
		t.Fatal("disabled core attempted startup", e)
	}
	a.lastPowerSwitch = time.Time{}
	if e = a.setOptirayEnabled(context.Background(), true); e == nil {
		t.Fatal("missing core accepted")
	}
	if optirayEnabled(a.store.snapshot()) {
		t.Fatal("failed enable lost disabled state")
	}
}
func TestOptiroutDisabledConfigurationSurvivesRestart(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.change(func(d Object) error { d["routing_engine"] = "optirout"; return nil }); e != nil {
		t.Fatal(e)
	}
	cfg, e := defaultOptirout()
	if e != nil {
		t.Fatal(e)
	}
	cfg["enabled"] = false
	a := &App{store: s}
	if e = a.saveRouting(context.Background(), cfg, false); e != nil {
		t.Fatal(e)
	}
	again, e := newStore(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	a.store = again
	if a.routingWanted(again.snapshot()) {
		t.Fatal("disabled routing restarts")
	}
	stored, e := readObject(filepath.Join(s.dir, "routing", "optirout.json"))
	if e != nil || configEnabled(stored) {
		t.Fatal("routing flag lost", e)
	}
	backup, _, e := s.backup(true, true)
	if e != nil {
		t.Fatal(e)
	}
	if configEnabled(obj(backup["optirout_config"])) {
		t.Fatal("backup lost routing disabled flag")
	}
}

func TestRoutingCanRunWithOptirayDisabled(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.change(func(d Object) error { d["routing_engine"] = "optirout"; d["optiray_enabled"] = false; return nil }); e != nil {
		t.Fatal(e)
	}
	cfg, e := defaultOptirout()
	if e != nil {
		t.Fatal(e)
	}
	cfg["enabled"] = true
	a := &App{store: s}
	if e = a.saveRouting(context.Background(), cfg, false); e != nil {
		t.Fatal(e)
	}
	if !a.routingWanted(s.snapshot()) {
		t.Fatal("manual routing is blocked by disabled core")
	}
	again, e := newStore(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	a.store = again
	if !a.routingWanted(again.snapshot()) || optirayEnabled(again.snapshot()) {
		t.Fatal("independent routing state lost")
	}
}

func TestRoutingPowerTransitionRestoresPreference(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		store, err := newStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := defaultOptirout()
		if err != nil {
			t.Fatal(err)
		}
		cfg["enabled"] = enabled
		if err := writeObject(filepath.Join(store.dir, "routing", "optirout.json"), cfg); err != nil {
			t.Fatal(err)
		}
		app := &App{store: store}
		before := store.snapshot()
		before["routing"] = Object{"enabled": true}
		off := clone(before)
		off["optiray_enabled"] = false
		extra, err := app.routingPowerTransition(before, off, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.persistExtra(off, extra); err != nil {
			t.Fatal(err)
		}
		on := clone(off)
		on["optiray_enabled"] = true
		extra, err = app.routingPowerTransition(off, on, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.persistExtra(on, extra); err != nil {
			t.Fatal(err)
		}
		restored, err := readObject(filepath.Join(store.dir, "routing", "optirout.json"))
		if err != nil {
			t.Fatal(err)
		}
		if configEnabled(restored) != enabled {
			t.Fatal("routing preference lost")
		}
		if !boolean(obj(on["routing"])["enabled"]) {
			t.Fatal("Xray routing preference lost")
		}
	}
}
