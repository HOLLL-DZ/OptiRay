package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func optiroutID(seen map[string]bool, id string) (string, error) {
	if regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(id) && !seen[id] {
		seen[id] = true
		return id, nil
	}
	for {
		var b [4]byte
		if _, e := rand.Read(b[:]); e != nil {
			return "", e
		}
		id = hex.EncodeToString(b[:])
		if !seen[id] {
			seen[id] = true
			return id, nil
		}
	}
}
func sanitizeOptirout(input Object) (Object, error) {
	cfg := clone(input)
	if v, ok := cfg["enabled"]; ok {
		if _, valid := v.(bool); !valid {
			return nil, fmt.Errorf("OptiRout enabled must be boolean")
		}
	}
	if e := normalizeOptirout(cfg); e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	groups := []any{}
	for _, v := range list(cfg["groups"]) {
		g := obj(v)
		if len(g) == 0 {
			return nil, fmt.Errorf("invalid routing group")
		}
		id, e := optiroutID(seen, str(g["id"]))
		if e != nil {
			return nil, e
		}
		enabled := true
		if v, exists := g["enable"]; exists {
			enabled = boolean(v)
		}
		out := Object{"id": id, "name": str(g["name"]), "color": first(str(g["color"]), "#888888"), "interface": str(g["interface"]), "enable": enabled}
		rules := []any{}
		for _, v := range list(g["rules"]) {
			r := obj(v)
			if len(r) == 0 {
				return nil, fmt.Errorf("invalid routing rule")
			}
			id, e := optiroutID(seen, str(r["id"]))
			if e != nil {
				return nil, e
			}
			enabled := true
			if v, exists := r["enable"]; exists {
				enabled = boolean(v)
			}
			rules = append(rules, Object{"id": id, "name": str(r["name"]), "type": first(str(r["type"]), "namespace"), "rule": str(r["rule"]), "enable": enabled})
		}
		out["rules"] = rules
		groups = append(groups, out)
	}
	out := Object{"configVersion": cfg["configVersion"], "app": cfg["app"], "groups": groups}
	if v, exists := cfg["enabled"]; exists {
		out["enabled"] = v
	}
	if v, exists := cfg["subscriptions"]; exists {
		out["subscriptions"] = v
	}
	return out, nil
}
func (s *Store) loadOptiroutLocked() (Object, error) {
	path := filepath.Join(s.dir, "routing", "optirout.json")
	cfg, e := readObject(path)
	if os.IsNotExist(e) {
		b, e := assets.ReadFile("assets/optirout-default.json")
		if e != nil {
			return nil, e
		}
		cfg = Object{}
		if e = json.Unmarshal(b, &cfg); e != nil {
			return nil, e
		}
		if e = normalizeOptirout(cfg); e != nil {
			return nil, e
		}
		if e = writeObject(path, cfg); e != nil {
			return nil, e
		}
	} else if e != nil {
		return nil, e
	}
	if e = normalizeOptirout(cfg); e != nil {
		return nil, e
	}
	return cfg, nil
}
func (s *Store) loadOptirout() (Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadOptiroutLocked()
}
func (s *Store) saveOptirout(cfg Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clean, e := sanitizeOptirout(cfg)
	if e != nil {
		return e
	}
	path := filepath.Join(s.dir, "routing", "optirout.json")
	previous, e := readObject(path)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if e == nil {
		oldJSON, _ := json.Marshal(previous)
		newJSON, _ := json.Marshal(clean)
		if string(oldJSON) == string(newJSON) {
			return nil
		}
		if boolean(s.data["optirout_save_backups"]) {
			name := backupName(str(s.data["proxy_name"]), "optirout_backup", "json", time.Now())
			name = strings.TrimSuffix(name, ".json") + time.Now().Format("_150405.000000000") + ".json"
			if e = writeObject(filepath.Join(s.dir, "routing", "backups", "optirout", name), previous); e != nil {
				return e
			}
		}
	}
	if e = writeObject(path, clean); e != nil {
		return e
	}
	if e = s.trimOptiroutBackupsLocked(10); e != nil {
		log.Printf("OptiRout backup retention: %v", e)
	}
	return nil
}

type routingBackup struct {
	Name  string
	Size  int64
	Mtime time.Time
}

func (s *Store) optiroutBackupsLocked() ([]routingBackup, error) {
	dir := filepath.Join(s.dir, "routing", "backups", "optirout")
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return []routingBackup{}, nil
	}
	if e != nil {
		return nil, e
	}
	items := []routingBackup{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return nil, e
		}
		items = append(items, routingBackup{entry.Name(), info.Size(), info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Mtime.Equal(items[j].Mtime) {
			return items[i].Name > items[j].Name
		}
		return items[i].Mtime.After(items[j].Mtime)
	})
	return items, nil
}
func (s *Store) trimOptiroutBackupsLocked(keep int) error {
	items, e := s.optiroutBackupsLocked()
	if e != nil {
		return e
	}
	for i := keep; i < len(items); i++ {
		if e = os.Remove(filepath.Join(s.dir, "routing", "backups", "optirout", items[i].Name)); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) optiroutBackups() (Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.optiroutBackupsLocked()
	if e != nil {
		return nil, e
	}
	result := []any{}
	for i, item := range items {
		if i >= 30 {
			break
		}
		result = append(result, Object{"name": item.Name, "size": item.Size, "mtime": item.Mtime.Unix()})
	}
	return Object{"ok": true, "backups": result, "dir": filepath.Join(s.dir, "routing", "backups", "optirout")}, nil
}
