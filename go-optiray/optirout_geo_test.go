package main

import "testing"

func TestOptiroutGeoSiteConversion(t *testing.T) {
	entry := geoMessage(10, []byte("TEST"))
	for _, item := range []struct {
		kind byte
		name string
	}{
		{0, "keyword"}, {1, ".*"}, {2, "example.com"}, {2, "example.com"}, {3, "exact.example.com"},
	} {
		record := append([]byte{8, item.kind}, geoMessage(18, []byte(item.name))...)
		record = append(record, geoMessage(26, geoMessage(10, []byte("ads")))...)
		entry = append(entry, geoMessage(18, record)...)
	}
	rules, skipped, err := geoRoutingRules(geoMessage(10, entry), "geosite", "test", "ads")
	if err != nil || skipped != 1 || len(rules) != 3 || rules[0]["rule"] != "*keyword*" || rules[1]["type"] != "namespace" || rules[2]["type"] != "domain" {
		t.Fatalf("rules = %#v, skipped = %d, error = %v", rules, skipped, err)
	}
	if _, _, err := geoRoutingRules(geoMessage(10, entry), "geosite", "test", "missing"); err == nil {
		t.Fatal("expected no rules for absent attribute")
	}
}

func TestOptiroutGeoIPConversion(t *testing.T) {
	entry := geoMessage(10, []byte("TEST"))
	v4 := append(geoMessage(10, []byte{192, 0, 2, 42}), 16, 24)
	v6addr := make([]byte, 16)
	copy(v6addr, []byte{0x20, 0x01, 0x0d, 0xb8})
	v6 := append(geoMessage(10, v6addr), 16, 32)
	entry = append(entry, geoMessage(18, v4)...)
	entry = append(entry, geoMessage(18, v6)...)
	rules, _, err := geoRoutingRules(geoMessage(10, entry), "geoip", "test", "")
	if err != nil || len(rules) != 2 || rules[0]["rule"] != "192.0.2.0/24" || rules[1]["rule"] != "2001:db8::/32" || rules[1]["type"] != "subnet6" {
		t.Fatalf("rules = %#v, error = %v", rules, err)
	}
	if _, _, err := geoRoutingRules(geoMessage(10, append(entry, 24, 1)), "geoip", "test", ""); err == nil {
		t.Fatal("inverted category imported as ordinary networks")
	}
}

func TestOptiroutGeoSourceSurvivesSave(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	cfg["groups"] = []any{Object{"geo_source": "geosite:test@ads", "rules": []any{Object{"type": "domain", "rule": "example.com"}}}}
	if err := s.saveOptirout(cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.loadOptirout()
	if err != nil {
		t.Fatal(err)
	}
	if got := obj(list(reloaded["groups"])[0])["geo_source"]; got != "geosite:test@ads" {
		t.Fatalf("geo source = %v", got)
	}
}
