package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Keep unknown settings and protocol fields when reading older backups.
type Object map[string]any
type Store struct {
	mu        sync.Mutex
	releaseMu sync.Mutex
	dir       string
	data      Object
}

func obj(v any) Object {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(Object); ok {
		return m
	}
	return Object{}
}
func list(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return []any{}
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func boolean(v any) bool { b, _ := v.(bool); return b }
func integer(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n, _ := strconv.Atoi(x)
		return n
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	}
	return 0
}
func clone(v any) Object { b, _ := json.Marshal(v); m := Object{}; _ = json.Unmarshal(b, &m); return m }
func readObject(path string) (Object, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	m := Object{}
	e = json.Unmarshal(b, &m)
	if m == nil && e == nil {
		e = errors.New("expected JSON object")
	}
	return m, e
}
func writeObject(path string, m Object) error {
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	return atomicBytes(path, append(b, '\n'))
}
func newStore(dir string) (*Store, error) {
	if e := recoverSettings(dir); e != nil {
		return nil, fmt.Errorf("settings recovery: %w", e)
	}
	b, e := assets.ReadFile("assets/defaults.json")
	if e != nil {
		return nil, e
	}
	d := Object{}
	if e = json.Unmarshal(b, &d); e != nil {
		return nil, e
	}
	raw, e := readObject(filepath.Join(dir, "subs.json"))
	if e == nil {
		for k, v := range raw {
			d[k] = v
		}
	} else if !os.IsNotExist(e) {
		return nil, fmt.Errorf("settings: %w", e)
	}
	// Settings are authoritative; the text file is a one-time legacy fallback.
	needsHWIDSave := strings.TrimSpace(str(raw["hwid"])) == ""
	if integer(d["logging_policy_version"]) < 1 {
		d["enable_logging"] = false
		d["logging_policy_version"] = 1
		needsHWIDSave = true
	}
	if strings.TrimSpace(str(d["hwid"])) == "" {
		hwid, readErr := os.ReadFile(filepath.Join(dir, "hwid.txt"))
		if readErr == nil {
			d["hwid"] = strings.TrimSpace(string(hwid))
			if str(d["hwid"]) == "" {
				return nil, fmt.Errorf("empty legacy HWID")
			}
		} else if !os.IsNotExist(readErr) {
			return nil, readErr
		}
	}
	if strings.TrimSpace(str(d["hwid"])) == "" {
		var id [16]byte
		if _, e = rand.Read(id[:]); e != nil {
			return nil, e
		}
		id[6] = (id[6] & 15) | 64
		id[8] = (id[8] & 63) | 128
		d["hwid"] = fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	}
	routing, e := readObject(filepath.Join(dir, "routing", "xray.json"))
	if e == nil {
		d["routing"] = routing
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if d["routing_engine"] == "magitrickle" {
		d["routing_engine"] = "optirout"
	}
	if v, ok := d["mt_save_backups"]; ok {
		d["optirout_save_backups"] = v
		delete(d, "mt_save_backups")
	}
	if e = migrateLoadedData(d); e != nil {
		return nil, e
	}
	d["schema_version"] = 3
	if strings.TrimSpace(str(d["hwid"])) == "" {
		return nil, fmt.Errorf("empty persisted HWID")
	}
	if needsHWIDSave {
		persisted := clone(d)
		delete(persisted, "routing")
		if e = writeObject(filepath.Join(dir, "subs.json"), persisted); e != nil {
			return nil, fmt.Errorf("persist HWID migration: %w", e)
		}
	}
	// Remove only after the identifier is safely stored in the common settings.
	if e = os.Remove(filepath.Join(dir, "hwid.txt")); e != nil && !os.IsNotExist(e) {
		return nil, fmt.Errorf("remove legacy HWID file: %w", e)
	}
	setDiagnosticLogging(dir, boolean(d["enable_logging"]))
	return &Store{dir: dir, data: d}, nil
}
func (s *Store) snapshot() Object { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.data) }
func (s *Store) change(fn func(Object) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.data)
	if e := fn(next); e != nil {
		return e
	}
	if e := s.persist(next); e != nil {
		return e
	}
	s.data = next
	return nil
}
func (s *Store) persist(d Object) error {
	return s.persistExtra(d, nil)
}
func (s *Store) persistExtra(d Object, extra map[string][]byte) error {
	persisted := clone(d)
	delete(persisted, "routing")
	settings, e := json.MarshalIndent(persisted, "", "  ")
	if e != nil {
		return e
	}
	routing, e := json.MarshalIndent(d["routing"], "", "  ")
	if e != nil {
		return e
	}
	files := map[string][]byte{"subs.json": settings, "routing/xray.json": routing}
	for k, v := range extra {
		files[k] = v
	}
	oldRouting, readErr := os.ReadFile(filepath.Join(s.dir, "routing", "xray.json"))
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	oldCompact, newCompact := &bytes.Buffer{}, &bytes.Buffer{}
	_ = json.Compact(oldCompact, oldRouting)
	_ = json.Compact(newCompact, routing)
	if readErr == nil && boolean(d["xray_save_backups"]) && !bytes.Equal(oldCompact.Bytes(), newCompact.Bytes()) {
		name := backupName(str(d["proxy_name"]), "xray_backup", "json", time.Now())
		name = strings.TrimSuffix(name, ".json") + time.Now().Format("_150405.000000000") + ".json"
		if e = atomicBytes(filepath.Join(s.dir, "routing", "backups", "xray", name), oldRouting); e != nil {
			return e
		}
	}
	if e = writeSettingsTransaction(s.dir, files); e != nil {
		return e
	}
	setDiagnosticLogging(s.dir, boolean(d["enable_logging"]))
	if e = trimBackupDirectory(filepath.Join(s.dir, "routing", "backups", "xray"), 10); e != nil {
		log.Printf("Xray backup retention: %v", e)
	}
	return nil
}
func findServer(d Object, sid, name string) (Object, Object) {
	for _, v := range list(d["subscriptions"]) {
		sub := obj(v)
		for _, x := range list(sub["servers"]) {
			srv := obj(x)
			if (sid != "" && str(srv["sid"]) == sid) || (sid == "" && name != "" && str(srv["name"]) == name) {
				return srv, sub
			}
		}
	}
	return nil, nil
}
