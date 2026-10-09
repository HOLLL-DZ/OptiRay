package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func validateRouting(r Object) error {
	for _, k := range []string{"block_rules", "direct_rules", "proxy_rules"} {
		if v, exists := r[k]; exists {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s must be text", k)
			}
		}
	}
	if v, exists := r["groups_ui"]; exists {
		if _, ok := v.(map[string]any); !ok {
			if _, ok = v.(Object); !ok {
				return fmt.Errorf("invalid routing groups")
			}
		}
		for _, k := range []string{"block", "direct", "proxy"} {
			if v, exists := obj(v)[k]; exists {
				if _, ok := v.([]any); !ok {
					return fmt.Errorf("invalid %s groups", k)
				}
			}
		}
	}
	return nil
}
func normalizeOptirout(r Object) error {
	if !strings.HasPrefix(str(r["configVersion"]), "0.") {
		return fmt.Errorf("unsupported OptiRout configuration")
	}
	if _, ok := r["groups"].([]any); !ok {
		return fmt.Errorf("OptiRout groups must be an array")
	}
	app := obj(r["app"])
	if r["app"] != nil && len(app) == 0 {
		if _, ok := r["app"].(map[string]any); !ok {
			return fmt.Errorf("invalid OptiRout app settings")
		}
	}
	web := obj(app["httpWeb"])
	web["enabled"] = false
	app["httpWeb"] = web
	nf := obj(app["netfilter"])
	iptables := obj(nf["iptables"])
	iptables["chainPrefix"] = "OR_"
	nf["iptables"] = iptables
	ipset := obj(nf["ipset"])
	ipset["tablePrefix"] = "or_"
	nf["ipset"] = ipset
	app["netfilter"] = nf
	r["app"] = app
	return nil
}
func (s *Store) backup(includeOptirout, includeXray bool) (Object, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := clone(s.data)
	delete(d, "optirout_config")
	kind := "settings"
	if includeOptirout {
		cfg, e := s.loadOptiroutLocked()
		if e != nil {
			return nil, "", fmt.Errorf("read OptiRout backup: %w", e)
		}
		if e = normalizeOptirout(cfg); e != nil {
			return nil, "", e
		}
		d["optirout_config"] = cfg
		kind += "+optirout"
	}
	if includeXray {
		kind += "+xrayrout"
	} else {
		delete(d, "routing")
	}
	return d, backupName(str(d["proxy_name"]), kind, "json", time.Now()), nil
}
func (s *Store) restore(payload Object) error {
	subscriptions, ok := payload["subscriptions"].([]any)
	if !ok {
		return fmt.Errorf("backup must contain subscriptions")
	}
	for _, v := range subscriptions {
		if _, ok := v.(map[string]any); !ok {
			return fmt.Errorf("invalid subscription")
		}
		sub := obj(v)
		if integer(sub["id"]) < 1 {
			return fmt.Errorf("invalid subscription ID")
		}
		if _, ok := sub["servers"].([]any); !ok {
			return fmt.Errorf("invalid server list")
		}
	}
	if routes, exists := payload["routing"]; exists {
		if _, ok := routes.(map[string]any); !ok {
			return fmt.Errorf("invalid Xray routing")
		}
		if e := validateRouting(obj(routes)); e != nil {
			return e
		}
	}
	if payload["optirout_config"] != nil {
		return fmt.Errorf("OptiRout restore must use the application runtime transaction")
	}
	return s.change(func(d Object) error {
		if str(d["selected"]) != "" {
			return fmt.Errorf("active core restore must use the application runtime transaction")
		}
		next := clone(payload)
		if _, exists := next["routing"]; !exists {
			next["routing"] = d["routing"]
		}
		delete(next, "optirout_config")
		for k := range d {
			delete(d, k)
		}
		defaults, _ := assets.ReadFile("assets/defaults.json")
		_ = json.Unmarshal(defaults, &d)
		for k, v := range next {
			d[k] = v
		}
		d["selected"] = ""
		d["schema_version"] = 3
		if str(d["hwid"]) == "" {
			return fmt.Errorf("backup HWID is empty")
		}
		return nil
	})
}
