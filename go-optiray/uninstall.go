package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Restore only recorded interfaces that still match the last settings we applied.
func (k *Keenetic) RemoveManaged(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	managed, e := readObject(filepath.Join(k.dir, "proxy-managed.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	original, e := readObject(filepath.Join(k.dir, "proxy-original.json"))
	if e != nil {
		return e
	}
	text, e := k.run(ctx, "show running-config")
	if e != nil {
		return e
	}
	proxies := parseProxies(text)
	// Preflight the entire journal before removing any interface.
	for id, v := range managed {
		if !regexp.MustCompile(`^Proxy[0-9]+$`).MatchString(id) {
			return fmt.Errorf("invalid managed interface")
		}
		p, exists := proxies[id]
		if !exists {
			continue
		}
		last := obj(v)
		if p.Name != str(last["proxy_name"]) || p.Host != str(last["proxy_host"]) || p.Port != integer(last["proxy_port"]) || p.Protocol != "socks5" {
			return fmt.Errorf("managed proxy %s was changed outside OptiRay; refusing deletion", id)
		}
		baseline, known := original[id]
		if !known {
			return fmt.Errorf("original state missing for %s", id)
		}
		if baseline != nil {
			if _, ok := baseline.(string); !ok {
				return fmt.Errorf("invalid proxy baseline")
			}
		}
	}
	for id, v := range managed {
		if !regexp.MustCompile(`^Proxy[0-9]+$`).MatchString(id) {
			return fmt.Errorf("invalid managed interface")
		}
		p, exists := proxies[id]
		if !exists {
			delete(managed, id)
			if e = writeObject(filepath.Join(k.dir, "proxy-managed.json"), managed); e != nil {
				return e
			}
			continue
		}
		last := obj(v)
		if p.Name != str(last["proxy_name"]) || p.Host != str(last["proxy_host"]) || p.Port != integer(last["proxy_port"]) || p.Protocol != "socks5" {
			return fmt.Errorf("managed proxy %s was changed outside OptiRay; refusing deletion", id)
		}
		baseline, known := original[id]
		if !known {
			return fmt.Errorf("original state missing for %s", id)
		}
		if baseline == nil {
			if _, e = k.run(ctx, "no interface "+id); e != nil {
				return e
			}
		} else {
			block, ok := baseline.(string)
			if !ok {
				return fmt.Errorf("invalid proxy baseline")
			}
			for _, line := range strings.Split(block, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if _, e = k.run(ctx, "interface "+id+" "+line); e != nil {
					return e
				}
			}
		}
		if _, e = k.run(ctx, "system configuration save"); e != nil {
			return e
		}
		delete(managed, id)
		if e = writeObject(filepath.Join(k.dir, "proxy-managed.json"), managed); e != nil {
			return e
		}
	}
	if _, e = k.run(ctx, "system configuration save"); e != nil {
		return e
	}
	return writeObject(filepath.Join(k.dir, "proxy-managed.json"), Object{})
}
