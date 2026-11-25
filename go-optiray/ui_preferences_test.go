package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUIPreferencesMigrationAndRestore(t *testing.T) {
	dir := t.TempDir()
	legacy, err := defaultData()
	if err != nil {
		t.Fatal(err)
	}
	legacy["hwid"] = "legacy-hwid"
	legacy["logging_policy_version"] = 1
	legacy["backup_include_optirout"] = true
	legacy["remember_server_browse"] = true
	legacy["server_browse"] = Object{"filter": "favorites", "sort": "name", "query": "old search"}
	legacy["subscriptions"] = []any{Object{"id": 1, "color": "#123456", "servers": []any{Object{"sid": "server-1", "name": "Server"}}}}
	legacy["favorites"] = []any{"server-1"}
	path := filepath.Join(dir, "subs.json")
	if err := writeObject(path, legacy); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	oldBackup, _, err := s.backup(true, true)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/save_sub_color", strings.NewReader("sub_id=1&color=%23abcdef"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	(&App{store: s}).ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("save subscription color: %d %s", res.Code, res.Body)
	}
	if err := s.setServerBrowse(Object{"filter": "all", "sort": "latency"}); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("legacy settings changed: %v", err)
	}
	reloaded, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	backup, _, err := reloaded.backup(true, true)
	if err != nil {
		t.Fatal(err)
	}
	expected := clone(oldBackup)
	obj(list(expected["subscriptions"])[0])["color"] = "#abcdef"
	expected["server_browse"] = Object{"filter": "all", "sort": "latency"}
	if !reflect.DeepEqual(clone(backup), clone(expected)) {
		t.Fatal("backup changed outside the expected interface preferences")
	}
	app := &App{store: reloaded}
	for _, payload := range []Object{oldBackup, backup} {
		if err := app.restoreState(context.Background(), payload); err != nil {
			t.Fatal(err)
		}
		reopened, err := newStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(uiPreferences(payload), uiPreferences(reopened.snapshot())) {
			t.Fatal("restored preferences did not survive restart")
		}
	}
}

func TestUIPreferencesWriteFailurePreservesSettings(t *testing.T) {
	dir := t.TempDir()
	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.change(func(d Object) error {
		d["subscriptions"] = []any{Object{"id": 1, "color": "#123456", "servers": []any{}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ui.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.setSubscriptionColor("1", "#abcdef"); err == nil {
		t.Fatal("expected a write failure")
	}
	reloaded, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, store := range []*Store{s, reloaded} {
		if color := obj(list(store.snapshot()["subscriptions"])[0])["color"]; color != "#123456" {
			t.Fatalf("color after failure = %v", color)
		}
	}
}

func TestOptiroutColorSaveDoesNotRequireProcessRestart(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	cfg["groups"] = []any{Object{"id": "abcdef01", "interface": "t2s0", "color": "#123456", "rules": []any{}}}
	if err := s.saveOptirout(cfg); err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	cfg, err = s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	group := obj(list(cfg["groups"])[0])
	group["color"] = "#abcdef"
	if err := a.saveRouting(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}
	persisted, err := s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	if obj(list(persisted["groups"])[0])["color"] != "#abcdef" {
		t.Fatal("color was not saved")
	}
	if err := a.saveRouting(context.Background(), persisted, true); err != nil {
		t.Fatal(err)
	}
	group["interface"] = "nwg0"
	if err := a.saveRouting(context.Background(), cfg, true); err == nil {
		t.Fatal("a routing interface change must require a process manager")
	}
}

func TestSettingsRecoveryRestoresUIPreferences(t *testing.T) {
	dir := t.TempDir()
	old := []byte("{\"subscription_colors\":{}}\n")
	journal, err := json.Marshal(Object{"ui.json": old})
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicBytes(filepath.Join(dir, ".settings-transaction.json"), journal); err != nil {
		t.Fatal(err)
	}
	if err := recoverSettings(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "ui.json"))
	if err != nil || !bytes.Equal(got, old) {
		t.Fatalf("preferences recovery failed: %v", err)
	}
}
