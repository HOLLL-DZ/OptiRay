package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptiroutDefaultsAndNormalization(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := s.loadOptirout()
	if e != nil {
		t.Fatal(e)
	}
	app := obj(cfg["app"])
	if boolean(obj(app["httpWeb"])["enabled"]) || str(obj(obj(app["netfilter"])["iptables"])["chainPrefix"]) != "OR_" {
		t.Fatal("unsafe routing defaults")
	}
	payload, name, e := s.backup(true, true)
	if e != nil || payload["optirout_config"] == nil || !strings.Contains(name, "+optirout+xrayrout") {
		t.Fatal(name, e)
	}
}
func TestOptiroutSanitizerDeduplicatesIDsAndRemovesUIState(t *testing.T) {
	cfg := Object{"configVersion": "0.8.2", "app": Object{}, "groups": []any{Object{"id": "deadbeef", "name": "First", "collapsed": true, "rules": []any{Object{"id": "deadbeef", "rule": "example.com", "selected": true}}}, Object{"id": "deadbeef", "name": "Second", "rules": []any{}}}}
	clean, e := sanitizeOptirout(cfg)
	if e != nil {
		t.Fatal(e)
	}
	groups := list(clean["groups"])
	first := obj(groups[0])
	rule := obj(list(first["rules"])[0])
	if first["collapsed"] != nil || rule["selected"] != nil || str(first["id"]) == str(rule["id"]) || str(first["id"]) == str(obj(groups[1])["id"]) {
		t.Fatal("invalid sanitized groups")
	}
	if len(str(rule["id"])) != 8 {
		t.Fatal("invalid generated ID")
	}
}
func TestOptiroutBackupRetention(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.change(func(d Object) error { d["optirout_save_backups"] = true; return nil }); e != nil {
		t.Fatal(e)
	}
	cfg, e := s.loadOptirout()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 12; i++ {
		obj(cfg["app"])["test_iteration"] = i
		if e = s.saveOptirout(cfg); e != nil {
			t.Fatal(e)
		}
	}
	items, e := s.optiroutBackups()
	if e != nil {
		t.Fatal(e)
	}
	if len(list(items["backups"])) != 10 {
		t.Fatal("retention failed", items)
	}
	current, e := s.loadOptirout()
	if e != nil || integer(obj(current["app"])["test_iteration"]) != 11 {
		t.Fatal("current rules lost")
	}
}
func TestOptiroutRestartRequestDoesNotWriteConfig(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	body := `{"config":{"configVersion":"0.8.2","groups":[]},"restart":true}`
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/optirout_config", strings.NewReader(body)))
	if w.Code != 503 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = os.Stat(filepath.Join(s.dir, "routing", "optirout.json")); !os.IsNotExist(e) {
		t.Fatal("blocked save wrote config")
	}
}
func TestOptiroutSaveAPIWithoutRestart(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	body := fmt.Sprintf(`{"config":{"configVersion":"0.8.2","groups":[]},"restart":false}`)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/optirout_config", strings.NewReader(body)))
	result := Object{}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || !boolean(result["ok"]) || boolean(result["restarted"]) {
		t.Fatal(w.Body.String())
	}
}
