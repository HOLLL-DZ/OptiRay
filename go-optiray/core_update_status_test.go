package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCoreVersionUpdateComparison(t *testing.T) {
	for _, tc := range []struct {
		next, current string
		want          bool
	}{
		{"26.10.22", "26.10.21", true}, {"26.10.21", "26.10.21", false}, {"26.9.30", "26.10.21", false},
		{"2.13.0", "2.9.0", true}, {"v2.13.0", "2.12.0", true}, {"2.13.0", "Неизвестно", false}, {"", "2.13.0", false},
		{"2.13.0", "2.13.0-beta.1", true}, {"2.13.0-beta.1", "2.13.0", false},
	} {
		if got := newerCoreVersion(tc.next, tc.current); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
func TestCoreUpdateIndicatorsUseCachedStableReleaseOnly(t *testing.T) {
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	fixture := &coreScheduleFixture{}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(s.dir, "bin")
	os.MkdirAll(root, 0700)
	path := filepath.Join(root, "xray")
	os.WriteFile(path, []byte("fixture"), 0600)
	stat, _ := os.Stat(path)
	app := &App{store: s}
	app.coreIndicators.versions = map[string]coreVersionStamp{"xray": {stat: stat, path: path, version: "26.10.21"}}
	writeObject(filepath.Join(s.dir, "xray_releases_cache.json"), Object{"recent": []any{Object{"version": "26.12.0", "is_pre": true}}, "stable": Object{"version": "26.11.0"}})
	info := app.coreUpdateStatus(context.Background())
	if !boolean(obj(info["xray"])["update_available"]) || str(obj(info["xray"])["latest"]) != "26.11.0" {
		t.Fatal(info)
	}
	if boolean(obj(info["hysteria"])["update_available"]) {
		t.Fatal("missing core reported as update")
	}
	if fixture.requests != 0 {
		t.Fatal("indicator polled GitHub")
	}
	os.Remove(path)
	info = app.coreUpdateStatus(context.Background())
	if boolean(obj(info["xray"])["update_available"]) {
		t.Fatal("removed core still shows update")
	}
}
