package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoutingEnvironmentSeparatesPaths(t *testing.T) {
	t.Setenv("OPTIRAY_ROUTING_PID", "foreign.pid")
	dir := t.TempDir()
	env := strings.Join(routingEnvironment(dir), "\n")
	if strings.Contains(env, "foreign.pid") {
		t.Fatal("inherited routing path leaked")
	}
	for _, entry := range []string{"OPTIRAY_ROUTING_PID=" + filepath.Join(dir, "routing", "engine.pid"), "OPTIRAY_ROUTING_SOCKET=" + filepath.Join(dir, "routing", "engine.sock"), "OPTIRAY_ROUTING_STATE_DIR=" + filepath.Join(dir, "routing")} {
		if !strings.Contains(env, entry) {
			t.Fatal("missing isolated path", entry)
		}
	}
}
func TestRoutingRefusesProductionPID(t *testing.T) {
	dir := t.TempDir()
	pid := filepath.Join(dir, "production.pid")
	if e := os.WriteFile(pid, []byte("123"), 0600); e != nil {
		t.Fatal(e)
	}
	p := &RoutingProcess{dir: dir, bin: "must-not-run", productionPID: pid}
	if e := p.Restart(context.Background()); e == nil {
		t.Fatal("parallel routing accepted")
	}
	if boolean(p.Status()["running"]) {
		t.Fatal("started process")
	}
}
func TestRoutingFailedRestartRestoresConfig(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	previous, e := s.loadOptirout()
	if e != nil {
		t.Fatal(e)
	}
	next := clone(previous)
	obj(next["app"])["test_change"] = true
	pid := filepath.Join(s.dir, "production.pid")
	_ = os.WriteFile(pid, []byte("123"), 0600)
	a := &App{store: s, routing: &RoutingProcess{dir: s.dir, bin: "must-not-run", productionPID: pid}}
	if e = a.saveRouting(context.Background(), next, true); e == nil {
		t.Fatal("restart failure ignored")
	}
	current, e := s.loadOptirout()
	if e != nil {
		t.Fatal(e)
	}
	if obj(current["app"])["test_change"] != nil {
		t.Fatal("configuration not rolled back")
	}
}
