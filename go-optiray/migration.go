package main

import (
	"fmt"
)

func migrateLoadedData(d Object) error {
	if _, ok := d["subscriptions"].([]any); !ok {
		return fmt.Errorf("invalid subscriptions settings")
	}
	ids := map[int]bool{}
	valid := map[string]bool{}
	byName := map[string][]string{}
	for i, v := range list(d["subscriptions"]) {
		sub := obj(v)
		if len(sub) == 0 {
			return fmt.Errorf("invalid subscription")
		}
		id := integer(sub["id"])
		if id < 1 {
			id = i + 1
		}
		if ids[id] {
			return fmt.Errorf("duplicate subscription ID")
		}
		ids[id] = true
		sub["id"] = id
		if sub["servers"] == nil {
			sub["servers"] = []any{}
		}
		if _, ok := sub["servers"].([]any); !ok {
			return fmt.Errorf("invalid server list")
		}
		for _, v := range list(sub["servers"]) {
			srv := obj(v)
			if len(srv) == 0 {
				return fmt.Errorf("invalid server")
			}
			if str(srv["sid"]) == "" {
				srv["sid"] = serverID(srv)
			}
			id := str(srv["sid"])
			valid[id] = true
			byName[str(srv["name"])] = append(byName[str(srv["name"])], id)
		}
	}
	selected := str(d["selected"])
	if selected != "" && !valid[selected] {
		if choices := byName[selected]; len(choices) == 1 {
			d["selected"] = choices[0]
		} else {
			d["selected"] = ""
		}
	}
	favs := []any{}
	seen := map[string]bool{}
	for _, v := range list(d["favorites"]) {
		id := str(v)
		if !valid[id] {
			if choices := byName[id]; len(choices) == 1 {
				id = choices[0]
			} else {
				continue
			}
		}
		if !seen[id] {
			seen[id] = true
			favs = append(favs, id)
		}
	}
	d["favorites"] = favs
	if str(d["subscription_client_profile"]) == "" {
		d["subscription_client_profile"] = "happ"
	}
	if d["routing_engine"] == "magitrickle" {
		d["routing_engine"] = "optirout"
	}
	r := obj(d["routing"])
	if r["block_ads"] != nil || r["direct_ru"] != nil || r["direct_private"] != nil {
		d["routing"] = Object{"enabled": boolean(r["enabled"]), "block_rules": "", "direct_rules": "", "proxy_rules": "", "direct_default": false, "groups_ui": Object{"block": []any{}, "direct": []any{}, "proxy": []any{}}}
	}
	return nil
}
