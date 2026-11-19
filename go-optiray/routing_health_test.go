package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestRoutingHealthReadsGuardInsteadOfProcessOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix routing socket exercised on router")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "routing")
	// Use a short temporary directory for AF_UNIX path length limits.
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(path, "engine.sock"))
	if err != nil {
		t.Fatal(err)
	}
	var ready atomic.Bool
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/system/routing-health" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(Object{"ready": ready.Load(), "paused": !ready.Load(), "source": "cli"})
	})}
	go server.Serve(listener)
	defer server.Close()
	health, err := routingHealth(dir)
	if err != nil || boolean(health["ready"]) {
		t.Fatalf("paused engine accepted: %v", err)
	}
	ready.Store(true)
	health, err = routingHealth(dir)
	if err != nil || !boolean(health["ready"]) {
		t.Fatalf("ready engine rejected: %v", err)
	}
}
