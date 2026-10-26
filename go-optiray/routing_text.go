package main

import (
	"fmt"
	"net"
	"strings"
)

func xrayRuleKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "regexp:") {
		return "regexp:" + value[len("regexp:"):]
	}
	if strings.HasPrefix(lower, "ip:") {
		lower = strings.TrimSpace(lower[3:])
	}
	if ip := net.ParseIP(lower); ip != nil {
		return "ip:" + ip.String()
	}
	if strings.HasPrefix(lower, "domain:") {
		lower = strings.TrimSpace(lower[7:])
	}
	if !strings.Contains(lower, ":") {
		lower = strings.TrimPrefix(lower, "*.")
		lower = strings.TrimRight(lower, ".")
	}
	return lower
}
func geoRulesFirst(value string) string {
	geo, other := []string{}, []string{}
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "geosite:") || strings.HasPrefix(lower, "geoip:") {
			geo = append(geo, line)
		} else {
			other = append(other, line)
		}
	}
	return strings.Join(append(geo, other...), "\n")
}
func prepareXrayRuleText(routing Object) error {
	seen := map[string]string{}
	groups := obj(routing["groups_ui"])
	for _, cat := range []string{"block", "direct", "proxy"} {
		lines := strings.Split(str(routing[cat+"_rules"]), "\n")
		for _, item := range list(groups[cat]) {
			group := obj(item)
			if enabled, ok := group["enabled"]; ok && !boolean(enabled) {
				continue
			}
			for _, item := range list(group["rules"]) {
				switch rule := item.(type) {
				case string:
					lines = append(lines, rule)
				default:
					value := obj(rule)
					if enabled, ok := value["enabled"]; ok && !boolean(enabled) {
						continue
					}
					lines = append(lines, str(value["value"]))
				}
			}
		}
		for _, line := range lines {
			key := xrayRuleKey(line)
			if key == "" {
				continue
			}
			if previous, ok := seen[key]; ok {
				if previous == cat {
					return fmt.Errorf("Повторяющееся правило: %s", strings.TrimSpace(line))
				}
				return fmt.Errorf("Правило задано в нескольких разделах: %s", strings.TrimSpace(line))
			}
			seen[key] = cat
		}
	}
	for _, cat := range []string{"block", "direct", "proxy"} {
		if value, ok := routing[cat+"_rules"].(string); ok {
			routing[cat+"_rules"] = geoRulesFirst(value)
		}
	}
	return nil
}
