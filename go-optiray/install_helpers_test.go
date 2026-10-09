package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshInstallUsesLANAddressAndUpdatesKeepSettings(t *testing.T) {
	address, e := chooseLANAddress([]lanCandidate{{"nwg0", "10.0.0.2"}, {"br0", "192.168.10.1"}})
	if e != nil || address != "192.168.10.1" {
		t.Fatal(address, e)
	}
	if _, e = chooseLANAddress([]lanCandidate{{"lo", "127.0.0.1"}, {"nwg0", "10.0.0.2"}}); e == nil {
		t.Fatal("used tunnel as LAN")
	}
	dir := t.TempDir()
	if e = initializeInstallation(dir, address); e != nil {
		t.Fatal(e)
	}
	d, e := readObject(filepath.Join(dir, "subs.json"))
	if e != nil {
		t.Fatal(e)
	}
	if d["proxy_host"] != address || d["routing_url"] != "http://192.168.10.1/controlPanel/policy" {
		t.Fatal(d["proxy_host"], d["routing_url"])
	}
	d["proxy_host"] = "custom.local"
	if e = writeObject(filepath.Join(dir, "subs.json"), d); e != nil {
		t.Fatal(e)
	}
	if e = initializeInstallation(dir, address); e != nil {
		t.Fatal(e)
	}
	d, _ = readObject(filepath.Join(dir, "subs.json"))
	if d["proxy_host"] != "custom.local" {
		t.Fatal("update overwrote user settings")
	}
}
func TestInstalledHealthAuthenticatesAndRequiresReady(t *testing.T) {
	dir := t.TempDir()
	if e := writeObject(filepath.Join(dir, "auth.json"), Object{"enabled": true, "user": "test", "pass": "secret"}); e != nil {
		t.Fatal(e)
	}
	ready := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "test" || p != "secret" {
			w.WriteHeader(401)
			return
		}
		jsonReply(w, 200, Object{"runtime": "go", "version": version, "ready": ready})
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")
	if e := checkInstalledService(context.Background(), dir, address); e == nil {
		t.Fatal("unready service accepted")
	}
	ready = true
	if e := checkInstalledService(context.Background(), dir, address); e != nil {
		t.Fatal(e)
	}
}
func TestLegacyProcessOwnership(t *testing.T) {
	if legacyProcessRole("/usr/bin/python3", "/opt/etc/optiray", []string{"python3", "app.py"}, false) != "manager" {
		t.Fatal("owned manager missed")
	}
	for _, args := range [][]string{{"xray", "run", "-c", "/opt/etc/other/config.json"}, {"python3", "/opt/etc/other/app.py"}} {
		if legacyProcessRole("/opt/bin/xray", "/opt/etc/other", args, false) != "" {
			t.Fatal("foreign process matched")
		}
	}
	if legacyProcessRole("/opt/bin/xray", "/", []string{"xray", "run", "-c", "/opt/etc/optiray/xray_config.json"}, false) != "core" {
		t.Fatal("owned core missed")
	}
	if legacyProcessRole("/opt/etc/optiray/bin/optiroutd", "/", []string{"optiroutd"}, false) != "" {
		t.Fatal("unjournaled engine matched")
	}
}

func TestLegacyEnvironmentAuthenticationIsPreserved(t *testing.T) {
	auth := authenticationFromEnvironment([]byte("OTHER=x\x00OPTIRAY_USER=admin\x00OPTIRAY_PASS=with spaces=ok\x00"))
	if str(auth["user"]) != "admin" || str(auth["pass"]) != "with spaces=ok" || !boolean(auth["enabled"]) {
		t.Fatal("legacy auth not preserved")
	}
	if authenticationFromEnvironment([]byte("OTHER=x\x00")) != nil {
		t.Fatal("enabled absent auth")
	}
}

func TestStoppedLegacyServicePreservesLiteralAuthentication(t *testing.T) {
	auth, e := legacyAuthenticationFromService("# OPTIRAY_USER=disabled\nexport OPTIRAY_USER=admin\nOPTIRAY_PASS='literal $ value'\n")
	if e != nil || str(auth["user"]) != "admin" || str(auth["pass"]) != "literal $ value" {
		t.Fatal("literal authentication lost", e)
	}
	if _, e = legacyAuthenticationFromService("OPTIRAY_USER=admin\nOPTIRAY_PASS=$(read-secret)\n"); e == nil {
		t.Fatal("evaluated shell authentication expression")
	}
}
