package main

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestXrayPowerPersistsBothDirectionsWithoutChangingRules(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.change(func(d Object) error {
		d["routing_engine"] = "xray"
		obj(d["routing"])["block_rules"] = "example.com"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	for _, enabled := range []bool{true, false, true} {
		if err = a.setRoutingPower(context.Background(), "xray", enabled); err != nil {
			t.Fatal(err)
		}
		again, err := newStore(s.dir)
		if err != nil {
			t.Fatal(err)
		}
		routes := obj(again.snapshot()["routing"])
		if boolean(routes["enabled"]) != enabled || str(routes["block_rules"]) != "example.com" {
			t.Fatal("power or saved rules changed incorrectly")
		}
		count := len(xrayRules(routes, "proxy", "outboundTag"))
		if enabled && count == 0 || !enabled && count != 0 {
			t.Fatal("power not reflected in generated Xray rules")
		}
	}
}

func TestOptiroutPowerStopsAndRollsBackFailedStart(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.change(func(d Object) error { d["routing_engine"] = "optirout"; return nil }); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	cfg["enabled"] = true
	if err = writeObject(filepath.Join(s.dir, "routing", "optirout.json"), cfg); err != nil {
		t.Fatal(err)
	}
	a := &App{store: s, routing: &RoutingProcess{dir: s.dir, bin: filepath.Join(s.dir, "missing-engine")}}
	if err = a.setRoutingPower(context.Background(), "optirout", false); err != nil {
		t.Fatal(err)
	}
	if a.routingWanted(s.snapshot()) {
		t.Fatal("disabled OptiRout still wanted")
	}
	if err = a.setRoutingPower(context.Background(), "optirout", true); err == nil {
		t.Fatal("failed startup accepted")
	}
	current, err := s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	if configEnabled(current) {
		t.Fatal("failed enable was not rolled back")
	}
}

func TestRoutingPowerRejectsStaleEngineAndInvalidPayload(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	engine := str(s.snapshot()["routing_engine"])
	other := "xray"
	if engine == other {
		other = "optirout"
	}
	if err = a.setRoutingPower(context.Background(), other, true); err == nil {
		t.Fatal("stale editor switched engines")
	}
	for _, body := range []string{`{"engine":"xray","enabled":"false"}`, `{"engine":"bad","enabled":true}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/routing_power", strings.NewReader(body))
		if !a.routingAPI(w, r) || w.Code != 400 {
			t.Fatalf("invalid payload accepted: %d", w.Code)
		}
	}
	if str(s.snapshot()["routing_engine"]) != engine {
		t.Fatal("invalid payload changed engine")
	}
}

func TestXrayPowerFailureRestoresSavedFlag(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.change(func(d Object) error {
		d["routing_engine"] = "xray"
		obj(d["routing"])["enabled"] = false
		d["selected"] = "test"
		d["subscriptions"] = []any{Object{"id": 1, "servers": []any{Object{"sid": "test", "outbound": Object{"protocol": "freedom"}}}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	if err = a.setRoutingPower(context.Background(), "xray", true); err == nil {
		t.Fatal("missing active core accepted")
	}
	if boolean(obj(s.snapshot()["routing"])["enabled"]) {
		t.Fatal("failed apply did not restore flag")
	}
}
