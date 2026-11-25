package netfilterTools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPolicyCLILiveReadOnly(t *testing.T) {
	if os.Getenv("OPTIRAY_TEST_LIVE_POLICY") != "1" {
		t.Skip("explicit router read-only test")
	}
	g := newPolicyGuard()
	scopes, err := g.loadCLI()
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) == 0 {
		t.Fatal("expected configured policies on test router")
	}
	for mark, allowed := range scopes {
		if mark == 0 || len(allowed) == 0 {
			t.Fatal("invalid live scope")
		}
	}
	t.Logf("read %d policies via CLI", len(scopes))
}

func TestPolicyCLIFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer srv.Close()
	g := newPolicyGuard()
	g.required = true
	g.baseURL = srv.URL
	g.client = srv.Client()
	config := "! $$$ Agent: http/rci\nip policy Policy0\n    permit global ISP\n    no permit global Proxy0\n    no permit global GigabitEthernet0/Vlan3\n!\nip policy Policy1\n    permit global Proxy0\n    permit global GigabitEthernet0/Vlan4\n!\n"
	state := "policy, name = Policy0, description = Test:\n mark: ffffaaa\npolicy, name = Policy1, description = Other:\n mark: ffffaab\n"
	g.runCLI = func(command string) (string, error) {
		switch command {
		case "show running-config":
			return config + "\x1b[K", nil
		case "show ip policy":
			return state, nil
		case "show interface ISP system-name":
			return "\n system-name: eth3\n", nil
		case "show interface Proxy0 system-name":
			return " system-name: t2s0\r\n\x1b[K", nil
		case "show interface GigabitEthernet0/Vlan4 system-name":
			return "system-name: eth0.4\n", nil
		default:
			return "", fmt.Errorf("unexpected CLI command")
		}
	}
	scopes, err := g.load()
	if err != nil {
		t.Fatal(err)
	}
	if g.source != "cli" || scopes[0xffffaaa]["t2s0"] || !scopes[0xffffaaa]["eth3"] || !scopes[0xffffaab]["t2s0"] || !scopes[0xffffaab]["eth0.4"] {
		t.Fatalf("bad scopes: %#v", scopes)
	}
	state = "policy, name = Policy0:\n mark: ffffaaa\n"
	if _, err = g.load(); err == nil {
		t.Fatal("partial runtime state accepted")
	}
	config = "NDM error: not ready"
	if _, err = g.load(); err == nil {
		t.Fatal("CLI error treated as empty policy set")
	}
}

func TestPolicyCLIRejectsUnknownPermissionsAndBadMarks(t *testing.T) {
	for _, name := range []string{"GigabitEthernet0//Vlan3", "GigabitEthernet0/Vlan3;show", "/Vlan3"} {
		if _, err := parseCLIPolicyConfig("ip policy Policy0\n permit global " + name + "\n"); err == nil {
			t.Errorf("invalid interface name accepted: %s", name)
		}
	}
	if _, err := parseCLIPolicyConfig("ip policy Policy0\n permit local Proxy0\n"); err == nil {
		t.Fatal("unsupported permission accepted")
	}
	for _, value := range []string{"garbage", "0", "fffffffff"} {
		if _, err := parseCLIPolicyState("policy, name = Policy0:\n mark: " + value); err == nil {
			t.Fatal("bad mark accepted")
		}
	}
	if _, err := parseCLIPolicyState("policy, name = Policy0:\n mark: ffffaaa\n mark: ffffaab"); err == nil {
		t.Fatal("duplicate mark accepted")
	}
	if _, err := parseCLIPolicyConfig("ip policy Policy0\nip policy Policy0\n"); err == nil {
		t.Fatal("duplicate config policy accepted")
	}
}

func TestPolicyHealthRejectsUnknownAndStaleStatus(t *testing.T) {
	g := newPolicyGuard()
	g.required = true
	nh := &Helper{policyGuard: g}
	if nh.PolicyHealth()["ready"] == true {
		t.Fatal("uninitialized guard ready")
	}
	g.ready = true
	g.checked = time.Now()
	g.source = "cli"
	if nh.PolicyHealth()["ready"] != true {
		t.Fatal("fresh guard not ready")
	}
	g.checked = time.Now().Add(-time.Minute)
	if nh.PolicyHealth()["ready"] == true {
		t.Fatal("stale guard ready")
	}
	if !strings.Contains(fmt.Sprint(nh.PolicyHealth()["paused"]), "true") {
		t.Fatal("stale guard not paused in status")
	}
}
