package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func syncOptiroutGeo(previous Object, dir string, names []string) (Object, error) {
	cfg := clone(previous)
	files := map[string][]byte{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		files[strings.TrimSuffix(name, ".dat")] = data
	}
	for _, item := range list(cfg["groups"]) {
		group := obj(item)
		source := str(group["geo_source"])
		kind, category, _ := strings.Cut(source, ":")
		data, updated := files[kind]
		if source == "" || !updated {
			continue
		}
		code, attr, _ := strings.Cut(category, "@")
		rules, skipped, err := geoRoutingRules(data, kind, code, attr)
		delete(group, "geo_notice")
		if err != nil {
			group["geo_notice"] = "Категория недоступна в текущей базе. Используются сохранённые правила."
			continue
		}
		if skipped > 0 {
			group["geo_notice"] = fmt.Sprintf("В базе есть неподдерживаемые записи (%d). Используются сохранённые правила.", skipped)
			continue
		}
		old := map[string]Object{}
		for _, entry := range list(group["rules"]) {
			r := obj(entry)
			old[str(r["type"])+":"+str(r["rule"])] = r
		}
		next := []any{}
		seen := map[string]bool{}
		for _, rule := range rules {
			if existing := old[str(rule["type"])+":"+str(rule["rule"])]; existing != nil {
				rule = clone(existing)
			} else {
				rule["id"], err = optiroutID(seen, "")
				if err != nil {
					return nil, err
				}
				rule["name"] = ""
				rule["enable"] = true
			}
			seen[str(rule["id"])] = true
			next = append(next, rule)
		}
		group["rules"] = next
	}
	return sanitizeOptirout(cfg)
}

func (s *Store) backupGeoRouting(before, after Object) error {
	if reflect.DeepEqual(before, after) {
		return nil
	}
	name := "geo_sync_" + time.Now().Format("20060102_150405.000000000") + ".json"
	return writeObject(filepath.Join(s.dir, "routing", "backups", "optirout", name), before)
}

func (a *App) syncSavedGeo() error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	previous, err := a.store.loadOptirout()
	if err != nil {
		return err
	}
	names := []string{}
	for _, name := range geoNames {
		if _, err := os.Stat(filepath.Join(a.store.dir, "bin", name)); err == nil {
			names = append(names, name)
		}
	}
	next, err := syncOptiroutGeo(previous, filepath.Join(a.store.dir, "bin"), names)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(previous, next) {
		return nil
	}
	if err = a.store.backupGeoRouting(previous, next); err != nil {
		return err
	}
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	before := a.store.snapshot()
	return a.commitState(context.Background(), before, clone(before), map[string][]byte{"routing/optirout.json": b})
}
