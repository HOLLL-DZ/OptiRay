package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func collectInterfaces(root, text string) ([]any, error) {
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil, e
	}
	names := map[string]bool{}
	aliases := map[string]string{}
	hints := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		names[name] = true
		if b, e := os.ReadFile(filepath.Join(root, name, "ifalias")); e == nil {
			alias := decodeInterfaceDescription(strings.TrimSpace(string(b)))
			if alias != "" {
				aliases[name] = alias
				hints[name] = alias
			}
		}
	}
	resolve := func(id string) string {
		lower := strings.ToLower(id)
		for _, pair := range [][2]string{{"proxy", "t2s"}, {"wireguard", "nwg"}, {"bridge", "br"}} {
			if m := regexp.MustCompile("^" + pair[0] + `([0-9]+)$`).FindStringSubmatch(lower); len(m) > 0 {
				if candidate := pair[1] + m[1]; names[candidate] {
					return candidate
				}
			}
		}
		for name, alias := range aliases {
			if strings.EqualFold(alias, id) {
				return name
			}
		}
		if names[lower] {
			return lower
		}
		return ""
	}
	id := ""
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "interface" {
			id = fields[1]
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			id = ""
		}
		if id != "" && len(fields) >= 2 && fields[0] == "description" {
			if name := resolve(id); name != "" {
				hints[name] = decodeInterfaceDescription(strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "description")), "\""))
			}
		}
	}
	skip := map[string]bool{}
	for _, name := range strings.Fields("lo sit0 tunl0 ip6tnl0 gre0 gretap0 erspan0 ip6gre0 ifb0 ifb1 dummy0") {
		skip[name] = true
	}
	hide := regexp.MustCompile(`(?i)^(apcli[0-9]+|ra[0-9]+(\.[0-9]+)?|wlan[0-9]+|eth[0-9]+(\.[0-9]+)?|ezcfg[0-9]+|ntce[0-9]+)$`)
	tunnel := regexp.MustCompile(`(?i)^(t2s|nwg|wg|vpn|tun|tap|proxy)[0-9]+$`)
	out := []any{}
	for name := range names {
		if skip[name] || hide.MatchString(name) || hints[name] == "" && !tunnel.MatchString(name) {
			continue
		}
		out = append(out, Object{"name": name, "desc": hints[name]})
	}
	sort.Slice(out, func(i, j int) bool { return str(obj(out[i])["name"]) < str(obj(out[j])["name"]) })
	return out, nil
}
func (a *App) interfaces(ctx context.Context) ([]any, error) {
	text := ""
	if a.keenetic != nil {
		var e error
		text, e = a.keenetic.run(ctx, "show running-config")
		if e != nil {
			return nil, e
		}
	}
	return collectInterfaces("/sys/class/net", text)
}

// NDMC prints non-ASCII description bytes as \xNN; decode once and keep invalid
// or incomplete byte sequences intact instead of producing broken UTF-8 labels.
func decodeInterfaceDescription(value string) string {
	decoded := regexp.MustCompile(`\\x[0-9a-fA-F]{2}`).ReplaceAllStringFunc(value, func(escape string) string {
		b, _ := strconv.ParseUint(escape[2:], 16, 8)
		return string([]byte{byte(b)})
	})
	if utf8.ValidString(decoded) {
		return decoded
	}
	return value
}
