package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func certPin(v any) string {
	if a := list(v); len(a) > 0 {
		v = a[0]
	}
	pin, ok := v.(string)
	if !ok {
		return ""
	}
	pin = regexp.MustCompile(`[^0-9a-fA-F]`).ReplaceAllString(pin, "")
	if len(pin) != 64 {
		return ""
	}
	return strings.ToLower(pin)
}
func normalizeOutbound(value any, index int) Object {
	ob := clone(value)
	if str(ob["protocol"]) == "" {
		return nil
	}
	if str(ob["tag"]) == "" {
		ob["tag"] = fmt.Sprintf("proxy_%d", index)
	}
	tls := obj(obj(ob["streamSettings"])["tlsSettings"])
	delete(tls, "allowInsecure")
	if tls["pinnedPeerCertSha256"] != nil {
		if pin := certPin(tls["pinnedPeerCertSha256"]); pin != "" {
			tls["pinnedPeerCertSha256"] = pin
		} else {
			delete(tls, "pinnedPeerCertSha256")
		}
	}
	return ob
}
func compositeServer(cfg Object) Object {
	rt := obj(cfg["routing"])
	balancers := list(cfg["balancers"])
	if len(balancers) == 0 {
		balancers = list(rt["balancers"])
	}
	if len(balancers) == 0 {
		return nil
	}
	outbounds := []any{}
	var firstProxy Object
	count := 0
	for i, v := range list(cfg["outbounds"]) {
		ob := normalizeOutbound(v, i)
		if ob == nil {
			continue
		}
		outbounds = append(outbounds, ob)
		p := str(ob["protocol"])
		if p != "freedom" && p != "blackhole" && p != "dns" {
			count++
			if firstProxy == nil {
				firstProxy = ob
			}
		}
	}
	if firstProxy == nil {
		return nil
	}
	host, port, _ := outboundInfo(firstProxy)
	if host == "" || port < 1 {
		return nil
	}
	name := first(str(cfg["remarks"]), str(cfg["name"]), fmt.Sprintf("Balancer (%d)", count))
	rules := []any{}
	for _, v := range list(rt["rules"]) {
		r := clone(v)
		if len(r) == 0 {
			continue
		}
		if _, ok := r["inboundTag"].([]any); ok {
			r["inboundTag"] = []any{"proxy0"}
		}
		rules = append(rules, r)
	}
	s := serverFromOutbound(name, host, port, "balancer", "balancer", "tls", nil, "JSON")
	s["composite"] = true
	s["candidates_count"] = count
	s["composite_data"] = Object{"outbounds": outbounds, "balancers": balancers, "dns": cfg["dns"], "observatory": cfg["observatory"], "routing_extra_rules": rules, "routing_domainStrategy": first(str(rt["domainStrategy"]), "IPIfNonMatch"), "routing_domainMatcher": first(str(rt["domainMatcher"]), "hybrid")}
	s["sid"] = serverID(s)
	return s
}
func canonicalServer(s Object) string {
	ob := clone(s["outbound"])
	delete(ob, "tag")
	if len(ob) == 0 {
		ob = clone(s)
		delete(ob, "sid")
		delete(ob, "name")
	}
	b, _ := json.Marshal(ob)
	return string(b)
}
