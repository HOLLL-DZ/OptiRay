package main

import (
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var geoRuleSource = regexp.MustCompile(`^(geosite|geoip):[a-z0-9_.-]+(?:@[a-z0-9_.-]+)?$`)

func geoRoutingRules(data []byte, kind, code, attribute string) ([]Object, int, error) {
	rules := []Object{}
	seen := map[string]bool{}
	skipped := 0
	found := false
	err := geoFields(data, func(field, wire uint64, entry []byte, _ uint64) error {
		if field != 1 || wire != 2 {
			return fmt.Errorf("Некорректная геобаза")
		}
		var name string
		var inverse bool
		var records [][]byte
		if err := geoFields(entry, func(field, wire uint64, value []byte, n uint64) error {
			switch {
			case field == 1 && wire == 2:
				name = string(value)
			case field == 2 && wire == 2:
				records = append(records, value)
			case field == 3 && wire == 0:
				inverse = n != 0
			}
			return nil
		}); err != nil {
			return err
		}
		if !strings.EqualFold(name, code) {
			return nil
		}
		found = true
		if kind == "geoip" && inverse {
			return fmt.Errorf("Инвертированная категория GeoIP не поддерживается")
		}
		for _, record := range records {
			if err := validateGeoRecord(record, kind+".dat"); err != nil {
				return err
			}
			ruleType, rule, matches, err := convertGeoRoutingRecord(record, kind, attribute)
			if err != nil {
				return err
			}
			if !matches {
				continue
			}
			if ruleType == "" {
				skipped++
			} else if key := ruleType + ":" + rule; !seen[key] {
				seen[key] = true
				rules = append(rules, Object{"type": ruleType, "rule": rule})
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if !found || len(rules) == 0 {
		return nil, skipped, fmt.Errorf("Категория %s не содержит подходящих правил", code)
	}
	return rules, skipped, nil
}

func convertGeoRoutingRecord(record []byte, kind, attribute string) (string, string, bool, error) {
	var raw []byte
	var text string
	var n uint64
	matches := attribute == ""
	err := geoFields(record, func(field, wire uint64, value []byte, number uint64) error {
		if kind == "geoip" {
			switch field {
			case 1:
				raw = value
			case 2:
				n = number
			}
			return nil
		}
		switch field {
		case 1:
			n = number
		case 2:
			text = string(value)
		case 3:
			if !matches && wire == 2 {
				return geoFields(value, func(f, w uint64, v []byte, _ uint64) error {
					if f == 1 && w == 2 && strings.EqualFold(string(v), attribute) {
						matches = true
					}
					return nil
				})
			}
		}
		return nil
	})
	if err != nil || !matches {
		return "", "", matches, err
	}
	if kind == "geoip" {
		addr, _ := netip.AddrFromSlice(raw)
		ruleType := "subnet"
		if addr.Is6() {
			ruleType = "subnet6"
		}
		return ruleType, netip.PrefixFrom(addr, int(n)).Masked().String(), true, nil
	}
	text = strings.ToLower(text)
	switch n {
	case 0:
		if !strings.ContainsAny(text, "*?") {
			return "wildcard", "*" + text + "*", true, nil
		}
	case 2:
		return "namespace", text, true, nil
	case 3:
		return "domain", text, true, nil
	}
	return "", "", true, nil
}

func (a *App) optiroutGeoRules(w http.ResponseWriter, r *http.Request) {
	source := strings.ToLower(r.URL.Query().Get("source"))
	if !geoRuleSource.MatchString(source) {
		jsonReply(w, 400, Object{"ok": false, "error": "Некорректная категория геобазы"})
		return
	}
	kind, category, _ := strings.Cut(source, ":")
	code, attribute, _ := strings.Cut(category, "@")
	if kind == "geoip" && attribute != "" {
		jsonReply(w, 400, Object{"ok": false, "error": "GeoIP не поддерживает атрибуты"})
		return
	}
	data, err := os.ReadFile(filepath.Join(a.store.dir, "bin", kind+".dat"))
	if err == nil {
		rules, skipped, convertErr := geoRoutingRules(data, kind, code, attribute)
		if convertErr == nil {
			jsonReply(w, 200, Object{"ok": true, "rules": rules, "skipped": skipped})
			return
		}
		err = convertErr
	}
	if os.IsNotExist(err) {
		err = fmt.Errorf("Файл %s.dat не установлен", kind)
	}
	jsonReply(w, 400, Object{"ok": false, "error": err.Error()})
}
