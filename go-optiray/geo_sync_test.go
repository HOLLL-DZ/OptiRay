package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGeoSyncPreservesMissingAndPartialGroups(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "geosite.dat"), geoSample("geosite.dat"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _ := defaultOptirout()
	cfg["groups"] = []any{
		Object{"id": "abcdef01", "name": "geosite:test", "geo_source": "geosite:test", "color": "#123456", "interface": "t2s0", "enable": false,
			"rules": []any{Object{"id": "abcdef02", "type": "namespace", "rule": "old.example", "enable": true}}},
		Object{"id": "abcdef03", "name": "geosite:missing", "geo_source": "geosite:missing", "interface": "nwg0", "enable": true,
			"rules": []any{Object{"id": "abcdef04", "type": "namespace", "rule": "keep.example", "enable": true}}},
	}
	cfg, _ = sanitizeOptirout(cfg)
	next, err := syncOptiroutGeo(cfg, dir, []string{"geosite.dat"})
	if err != nil {
		t.Fatal(err)
	}
	first := obj(list(next["groups"])[0])
	second := obj(list(next["groups"])[1])
	if first["enable"] != false || first["color"] != "#123456" || first["interface"] != "t2s0" || obj(list(first["rules"])[0])["rule"] != "example.com" {
		t.Fatalf("unexpected group: %#v", first)
	}
	if second["geo_notice"] == nil || !reflect.DeepEqual(clone(Object{"rules": second["rules"]}), clone(Object{"rules": obj(list(cfg["groups"])[1])["rules"]})) {
		t.Fatal("missing category lost rules")
	}
	record := append([]byte{8, 1}, geoMessage(18, []byte(".*"))...)
	entry := append(geoMessage(10, []byte("TEST")), geoMessage(18, record)...)
	entry = append(entry, geoMessage(18, append([]byte{8, 2}, geoMessage(18, []byte("new.example"))...))...)
	os.WriteFile(filepath.Join(dir, "geosite.dat"), geoMessage(10, entry), 0600)
	partial, err := syncOptiroutGeo(next, dir, []string{"geosite.dat"})
	if err != nil {
		t.Fatal(err)
	}
	g := obj(list(partial["groups"])[0])
	if g["geo_notice"] == nil || !reflect.DeepEqual(g["rules"], first["rules"]) {
		t.Fatal("partial conversion replaced working rules")
	}
	os.WriteFile(filepath.Join(dir, "geosite.dat"), geoSample("geosite.dat"), 0600)
	recovered, err := syncOptiroutGeo(partial, dir, []string{"geosite.dat"})
	if err != nil || obj(list(recovered["groups"])[0])["geo_notice"] != nil {
		t.Fatal("notice did not clear", err)
	}
}

func TestGeoUpdateRollsBackDatabaseAndRulesOnRuntimeFailure(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	http.DefaultClient = &http.Client{Transport: &geoFixtureTransport{body: geoSample("geosite.dat")}}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.loadOptirout()
	cfg["groups"] = []any{Object{"id": "abcdef01", "name": "geosite:test", "geo_source": "geosite:test", "interface": "t2s0", "enable": true, "rules": []any{Object{"id": "abcdef02", "type": "namespace", "rule": "old.example", "enable": true}}}}
	if err = s.saveOptirout(cfg); err != nil {
		t.Fatal(err)
	}
	old, _ := s.loadOptirout()
	root := filepath.Join(s.dir, "bin")
	os.MkdirAll(root, 0700)
	path := filepath.Join(root, "geosite.dat")
	os.WriteFile(path, []byte("old database"), 0600)
	s.change(func(d Object) error {
		d["subscriptions"] = []any{Object{"id": 1, "servers": []any{Object{"sid": "selected", "name": "selected"}}}}
		d["selected"] = "selected"
		d["optiray_enabled"] = true
		return nil
	})
	before := s.snapshot()
	a := &App{store: s}
	err = a.updateGeo(context.Background(), "github", "geosite.dat", "https://example.com/geosite.dat")
	if err == nil {
		t.Fatal("expected runtime failure without core manager")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "old database" {
		t.Fatal("database not restored")
	}
	got, _ := s.loadOptirout()
	if !reflect.DeepEqual(got, old) {
		t.Fatal("rules not restored")
	}
	if !reflect.DeepEqual(before, s.snapshot()) {
		t.Fatal("settings changed on failure")
	}
}
