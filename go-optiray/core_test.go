package main

import (
	"net"
	"testing"
)

func TestCoreRefusesOccupiedPort(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	c := &CoreManager{bin: "must-not-execute", dir: t.TempDir()}
	if e = c.Apply(Object{}, port); e == nil {
		t.Fatal("occupied port accepted")
	}
	if c.cmd != nil {
		t.Fatal("another process was touched")
	}
}
func TestCompositeRoutingPreservesDNSAndBalancer(t *testing.T) {
	s := Object{"composite": true, "composite_data": Object{"outbounds": []any{Object{"tag": "candidate", "protocol": "vless"}}, "balancers": []any{Object{"tag": "pool", "selector": []any{"candidate"}}}, "dns": Object{"servers": []any{"1.1.1.1"}}}}
	d := Object{"routing_engine": "xray", "routing": Object{"enabled": true, "direct_default": true, "block_rules": "*.ads.example\nip:10.0.0.0/8", "proxy_rules": "example.org"}}
	cfg, e := xrayConfig(s, d, 3090)
	if e != nil {
		t.Fatal(e)
	}
	rt := obj(cfg["routing"])
	rules := list(rt["rules"])
	if len(list(rt["balancers"])) != 1 || cfg["dns"] == nil {
		t.Fatal("composite metadata lost")
	}
	if str(obj(rules[2])["balancerTag"]) != "pool" || str(obj(rules[3])["outboundTag"]) != "direct" {
		t.Fatal(rules)
	}
}
func TestOptiroutModeDoesNotApplyXrayUserRules(t *testing.T) {
	srv := Object{"outbound": Object{"tag": "proxy", "protocol": "vless"}}
	cfg, e := xrayConfig(srv, Object{"routing_engine": "optirout", "routing": Object{"enabled": true, "block_rules": "example.com"}}, 3090)
	if e != nil {
		t.Fatal(e)
	}
	rules := list(obj(cfg["routing"])["rules"])
	if len(rules) != 1 || str(obj(rules[0])["outboundTag"]) != "proxy" {
		t.Fatal(rules)
	}
}
