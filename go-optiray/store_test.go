package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSettingsRoundTripPreservesFields(t *testing.T) {
	dir := t.TempDir()
	initial := Object{"custom_future_setting": Object{"enabled": true}, "selected": "active-sid", "hwid": "existing-hwid", "subscriptions": []any{Object{"id": 7, "name": "Test", "servers": []any{Object{"sid": "active-sid", "outbound": Object{"protocol": "vless", "custom": true}}}}}}
	if e := writeObject(filepath.Join(dir, "subs.json"), initial); e != nil {
		t.Fatal(e)
	}
	s, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.change(func(d Object) error { d["proxy_name"] = "OptiRay_Admin"; return nil }); e != nil {
		t.Fatal(e)
	}
	again, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	d := again.snapshot()
	if str(d["hwid"]) != "existing-hwid" || str(d["selected"]) != "active-sid" || !boolean(obj(d["custom_future_setting"])["enabled"]) {
		t.Fatal("settings lost")
	}
	srv, _ := findServer(d, "active-sid", "")
	if !boolean(obj(srv["outbound"])["custom"]) {
		t.Fatal("outbound field lost")
	}
}

func TestFreshDefaultsAndExistingPreferences(t *testing.T) {
	fresh, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := fresh.snapshot()
	if str(d["auto_select_source"]) != "favorites" {
		t.Fatal("fresh installation must select favorites")
	}
	for _, key := range []string{"enable_logging", "optirout_save_backups", "xray_save_backups"} {
		if boolean(d[key]) {
			t.Fatalf("fresh installation enables %s", key)
		}
	}
	dir := t.TempDir()
	if err := writeObject(filepath.Join(dir, "subs.json"), Object{
		"hwid": "existing", "auto_select_source": "all", "enable_logging": true,
		"optirout_save_backups": true, "xray_save_backups": true,
	}); err != nil {
		t.Fatal(err)
	}
	existing, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	d = existing.snapshot()
	if str(d["auto_select_source"]) != "all" || !boolean(d["enable_logging"]) ||
		!boolean(d["optirout_save_backups"]) || !boolean(d["xray_save_backups"]) {
		t.Fatal("existing preferences overwritten by new defaults")
	}
}
func TestCorruptSettingsAreNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subs.json")
	_ = os.WriteFile(path, []byte("{broken"), 0600)
	if _, e := newStore(dir); e == nil {
		t.Fatal("invalid settings accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "{broken" {
		t.Fatal("corrupt settings overwritten")
	}
}
func TestBackupFilename(t *testing.T) {
	got := backupName("OptiRay_Admin", "settings+optirout+xrayrout", "json", time.Date(2026, 10, 8, 9, 2, 0, 0, time.UTC))
	want := "OptiRay_Admin-settings+optirout+xrayrout_08-10-2026_0902.json"
	if got != want {
		t.Fatalf("%s", got)
	}
}
