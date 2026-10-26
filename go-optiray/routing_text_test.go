package main

import (
	"strings"
	"testing"
)

func TestXrayGeoTextStableSort(t *testing.T) {
	original := "example.com\ngeosite:youtube\n# note\ngeoip:ru\n192.0.2.1"
	want := "geosite:youtube\ngeoip:ru\nexample.com\n# note\n192.0.2.1"
	if got := geoRulesFirst(original); got != want {
		t.Fatal(got)
	}
	routing := Object{"block_rules": original}
	if err := prepareXrayRuleText(routing); err != nil {
		t.Fatal(err)
	}
	if str(routing["block_rules"]) != want {
		t.Fatal("not sorted after preparation")
	}
}
func TestXrayDuplicatesAcrossFieldsGroupsAndOutbounds(t *testing.T) {
	for _, routing := range []Object{
		{"block_rules": "geosite:youtube\nGEOSITE:YOUTUBE"},
		{"block_rules": "example.com", "direct_rules": "domain:EXAMPLE.COM."},
		{"proxy_rules": "geoip:ru", "groups_ui": Object{"proxy": []any{Object{"rules": []any{Object{"value": "GEOIP:RU", "enabled": true}}}}}},
		{"block_rules": "ip:192.0.2.1", "proxy_rules": "192.0.2.1"},
	} {
		if err := prepareXrayRuleText(routing); err == nil {
			t.Fatal("duplicate accepted", routing)
		}
	}
	routing := Object{"block_rules": "geosite:youtube", "groups_ui": Object{"direct": []any{Object{"enabled": false, "rules": []any{"geosite:youtube"}}, Object{"rules": []any{Object{"value": "geosite:youtube", "enabled": false}}}}}}
	if err := prepareXrayRuleText(routing); err != nil {
		t.Fatal("disabled rules treated as active duplicates", err)
	}
	if xrayRuleKey("regexp:Example") == xrayRuleKey("regexp:example") {
		t.Fatal("case-sensitive regex conflated")
	}
	if !strings.HasPrefix(geoRulesFirst("domain:example.com\n geosite:youtube"), " geosite:youtube") {
		t.Fatal("geo rule not first")
	}
}
