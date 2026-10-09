package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Caller owns the production stop/restore trap. This test touches only the separate manager.
func TestRouterFullSwitch(t *testing.T) {
	root := os.Getenv("OPTIRAY_FULL_FIXTURE")
	if root == "" {
		t.Skip("full switch not enabled")
	}
	d, e := readObject(filepath.Join(root, "subs.json"))
	if e != nil {
		t.Fatal(e)
	}
	client := &http.Client{Timeout: 25 * time.Second}
	request := func(method, path string, body []byte, kind string) Object {
		t.Helper()
		req, e := http.NewRequest(method, "http://127.0.0.1:3884"+path, bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		if kind != "" {
			req.Header.Set("Content-Type", kind)
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		b, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if e != nil {
			t.Fatal(e)
		}
		var out Object
		if e = json.Unmarshal(b, &out); e != nil {
			t.Fatalf("%s invalid JSON", path)
		}
		if res.StatusCode != 200 || out["error"] != nil {
			t.Fatalf("%s HTTP %d: %v", path, res.StatusCode, out["error"])
		}
		return out
	}
	deadline := time.Now().Add(25 * time.Second)
	for {
		res, e := client.Get("http://127.0.0.1:3884/api/health")
		if e == nil {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("manager timeout")
		}
		time.Sleep(200 * time.Millisecond)
	}
	for {
		if boolean(request("GET", "/api/optirout_status", nil, "")["running"]) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("routing timeout")
		}
		time.Sleep(300 * time.Millisecond)
	}
	if str(d["selected"]) == "" {
		t.Fatal("no saved selection")
	}
	form := url.Values{"sid": {str(d["selected"])}}
	out := request("POST", "/select", []byte(form.Encode()), "application/x-www-form-urlencoded")
	if str(out["warning"]) != "" {
		t.Fatalf("connection warning: %v", out["warning"])
	}
	t.Log("saved server and Keenetic proxy verified")

	// A real DNS query through the engine exercises the listener, upstream and rule hooks.
	cfgDNS, err := readObject(filepath.Join(root, "routing", "optirout.json"))
	if err != nil {
		t.Fatal(err)
	}
	dnsPort := integer(obj(obj(obj(cfgDNS["app"])["dnsProxy"])["host"])["port"])
	dns, err := net.DialTimeout("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(dnsPort)), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer dns.Close()
	_ = dns.SetDeadline(time.Now().Add(7 * time.Second))
	query := []byte{0x7b, 0x51, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 7, 'y', 'o', 'u', 't', 'u', 'b', 'e', 3, 'c', 'o', 'm', 0, 0, 1, 0, 1}
	if _, err = dns.Write(query); err != nil {
		t.Fatal(err)
	}
	answer := make([]byte, 4096)
	n, err := dns.Read(answer)
	if err != nil {
		t.Fatal(err)
	}
	if n < 12 || answer[0] != query[0] || answer[1] != query[1] || answer[2]&0x80 == 0 || answer[3]&15 != 0 || answer[6] == 0 && answer[7] == 0 {
		t.Fatal("invalid engine DNS response")
	}
	t.Log("engine DNS upstream and group hooks passed")

	probe := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if ms := httpPing(ctx, integer(d["proxy_port"]), "http://cp.cloudflare.com/generate_204", 10*time.Second); ms < 0 {
			t.Fatal("active proxy HTTP failed")
		} else {
			t.Logf("active proxy HTTP %d ms", ms)
		}
	}
	probe()
	cfg := request("GET", "/api/optirout_config", nil, "")["config"]
	body, _ := json.Marshal(Object{"config": cfg, "restart": true})
	request("POST", "/api/optirout_config", body, "application/json")
	request("POST", "/api/optirout_restart", nil, "")
	t.Log("routing save/restart passed")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e = netfilterHook(ctx, root, "iptables", "mangle"); e != nil {
		t.Fatal(e)
	}
	t.Log("native firmware hook passed")
	probe()
	body, _ = json.Marshal(Object{"engine": "xray"})
	request("POST", "/api/routing_rules_mode", body, "application/json")
	if boolean(request("GET", "/api/optirout_status", nil, "")["running"]) {
		t.Fatal("OptiRout remained running")
	}
	probe()
	body, _ = json.Marshal(Object{"engine": "optirout"})
	request("POST", "/api/routing_rules_mode", body, "application/json")
	if !boolean(request("GET", "/api/optirout_status", nil, "")["running"]) {
		t.Fatal("OptiRout did not return")
	}
	probe()
	t.Log("routing modes round trip passed")
}
