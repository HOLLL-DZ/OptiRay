package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func applySettings(d Object, form url.Values) error {
	for _, key := range []string{"proxy_name", "proxy_host", "proxy_port"} {
		if form.Has(key) {
			if key == "proxy_port" {
				v, e := strconv.Atoi(form.Get(key))
				if e != nil || v < 1 || v > 65535 {
					return fmt.Errorf("invalid proxy port")
				}
				d[key] = v
			} else {
				d[key] = strings.TrimSpace(form.Get(key))
			}
		}
	}
	if !validProxyParams(str(d["proxy_name"]), str(d["proxy_host"]), integer(d["proxy_port"])) {
		return fmt.Errorf("invalid proxy settings")
	}
	for _, key := range []string{"routing_url", "test_url", "test_button_url", "hwid"} {
		if form.Has(key) {
			v := strings.TrimSpace(form.Get(key))
			if key == "hwid" && v == "" {
				return fmt.Errorf("empty HWID")
			}
			d[key] = v
		}
	}
	if form.Has("subscription_client_profile") {
		p := form.Get("subscription_client_profile")
		if p != "happ" && p != "incy" && p != "optiray" {
			return fmt.Errorf("invalid subscription client")
		}
		d["subscription_client_profile"] = p
	}
	bounds := map[string][2]float64{"ping_timeout": {0.5, 30}, "ping_tcp_concurrency": {1, 20}, "ping_real_concurrency": {1, 10}, "ping_sample_delay_ms": {0, 5000}}
	for key, bound := range bounds {
		if form.Has(key) {
			v, e := strconv.ParseFloat(form.Get(key), 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < bound[0] || v > bound[1] {
				return fmt.Errorf("invalid %s", key)
			}
			if key != "ping_timeout" && v != float64(int(v)) {
				return fmt.Errorf("%s must be an integer", key)
			}
			d[key] = v
		}
	}
	for _, key := range []string{"show_br0", "enable_logging", "show_ping_all", "show_test_button", "show_test_float", "update_force_restart", "backup_include_optirout", "backup_include_xray"} {
		d[key] = form.Has(key)
	}
	if form.Has("auto_select_source") {
		v := form.Get("auto_select_source")
		if v != "all" && v != "favorites" {
			return fmt.Errorf("invalid auto select source")
		}
		d["auto_select_source"] = v
	}
	if form.Has("routing_engine") {
		v := form.Get("routing_engine")
		if v != "xray" && v != "optirout" {
			return fmt.Errorf("invalid routing engine")
		}
		d["routing_engine"] = v
	}
	d["routing_use_external"] = form.Get("routing_use_external") == "1"
	routes := obj(d["routing"])
	if form.Has("routing_block_rules") || form.Has("routing_groups_json") {
		routes["enabled"] = form.Has("routing_enabled")
	}
	if form.Has("routing_mode") {
		routes["direct_default"] = form.Get("routing_mode") == "direct_default"
	}
	for _, key := range []string{"block_rules", "direct_rules", "proxy_rules"} {
		if form.Has("routing_" + key) {
			routes[key] = form.Get("routing_" + key)
		}
	}
	if form.Has("routing_groups_json") {
		groups := Object{}
		if e := json.Unmarshal([]byte(form.Get("routing_groups_json")), &groups); e != nil {
			return fmt.Errorf("invalid routing groups JSON")
		}
		routes["groups_ui"] = groups
	}
	if e := validateRouting(routes); e != nil {
		return e
	}
	d["routing"] = routes
	return nil
}
