package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// A durable rollback journal protects multi-file settings updates. On startup an
// interrupted update is restored before any configuration is read.
func atomicBytes(path string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".optiray-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return e
	}
	return syncDirectory(filepath.Dir(path))
}
func recoverSettings(dir string) error {
	path := filepath.Join(dir, ".settings-transaction.json")
	journal, e := readObject(path)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for name, v := range journal {
		if name != "subs.json" && name != "hwid.txt" && name != "auth.json" && name != "routing/xray.json" && name != "routing/optirout.json" {
			return fmt.Errorf("invalid recovery path")
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if v == nil {
			if e = os.Remove(target); e != nil && !os.IsNotExist(e) {
				return e
			}
		} else {
			b, e := json.Marshal(v)
			if e != nil {
				return e
			}
			var bytes []byte
			if e = json.Unmarshal(b, &bytes); e != nil {
				return e
			}
			if e = atomicBytes(target, bytes); e != nil {
				return e
			}
		}
	}
	return os.Remove(path)
}
func writeSettingsTransaction(dir string, files map[string][]byte) error {
	if e := recoverSettings(dir); e != nil {
		return e
	}
	journal := Object{}
	names := []string{}
	for name := range files {
		if name != "subs.json" && name != "hwid.txt" && name != "auth.json" && name != "routing/xray.json" && name != "routing/optirout.json" {
			return fmt.Errorf("invalid transaction path")
		}
		b, e := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if e == nil {
			journal[name] = b
		} else if os.IsNotExist(e) {
			journal[name] = nil
		} else {
			return e
		}
		names = append(names, name)
	}
	sort.Strings(names)
	path := filepath.Join(dir, ".settings-transaction.json")
	if e := writeObject(path, journal); e != nil {
		return e
	}
	for _, name := range names {
		if e := atomicBytes(filepath.Join(dir, filepath.FromSlash(name)), files[name]); e != nil {
			if rollback := recoverSettings(dir); rollback != nil {
				return fmt.Errorf("settings write failed; rollback failed: %w", rollback)
			}
			return e
		}
	}
	if e := os.Remove(path); e != nil {
		if rollback := recoverSettings(dir); rollback != nil {
			return fmt.Errorf("commit failed and rollback failed: %w", rollback)
		}
		return e
	}
	return nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, e := os.Open(path)
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
