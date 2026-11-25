package main

import (
	"encoding/json"
	"fmt"
	"io"
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
		if name != "bin/geosite.dat" && name != "bin/geoip.dat" && name != "subs.json" && name != "ui.json" && name != "hwid.txt" && name != "auth.json" && name != "routing/xray.json" && name != "routing/optirout.json" {
			return fmt.Errorf("invalid recovery path")
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if v == nil {
			if e = os.Remove(target); e != nil && !os.IsNotExist(e) {
				return e
			}
		} else if ref := str(obj(v)["file"]); ref != "" {
			expected := ".settings-backup-" + filepath.Base(name)
			if ref != expected || (name != "bin/geoip.dat" && name != "bin/geosite.dat") {
				return fmt.Errorf("invalid recovery file")
			}
			if e = copyTransactionFile(filepath.Join(dir, ref), target); e != nil {
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
	if e := os.Remove(path); e != nil {
		return e
	}
	return syncDirectory(dir)
}
func writeSettingsTransaction(dir string, files map[string][]byte, diskFiles ...map[string]string) error {
	sources := map[string]string{}
	if len(diskFiles) > 0 {
		sources = diskFiles[0]
	}
	for name := range sources {
		if name != "bin/geoip.dat" && name != "bin/geosite.dat" {
			return fmt.Errorf("invalid disk file")
		}
		files[name] = nil
	}
	if e := recoverSettings(dir); e != nil {
		return e
	}
	journal := Object{}
	names := []string{}
	for name := range files {
		if name != "bin/geosite.dat" && name != "bin/geoip.dat" && name != "subs.json" && name != "ui.json" && name != "hwid.txt" && name != "auth.json" && name != "routing/xray.json" && name != "routing/optirout.json" {
			return fmt.Errorf("invalid transaction path")
		}
		if name == "bin/geoip.dat" || name == "bin/geosite.dat" {
			ref := ".settings-backup-" + filepath.Base(name)
			e := copyTransactionFile(filepath.Join(dir, filepath.FromSlash(name)), filepath.Join(dir, ref))
			if os.IsNotExist(e) {
				journal[name] = nil
			} else if e != nil {
				return e
			} else {
				journal[name] = Object{"file": ref}
			}
			names = append(names, name)
			continue
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
		target := filepath.Join(dir, filepath.FromSlash(name))
		var e error
		if source := sources[name]; source != "" {
			e = copyTransactionFile(source, target)
		} else if files[name] == nil {
			e = os.Remove(target)
			if os.IsNotExist(e) {
				e = nil
			}
			if e == nil {
				e = syncDirectory(filepath.Dir(target))
			}
		} else {
			e = atomicBytes(target, files[name])
		}
		if e != nil {
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
	return syncDirectory(dir)
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

func copyTransactionFile(source, target string) error {
	in, e := os.Open(source)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.CreateTemp(filepath.Dir(target), ".transaction-copy-*")
	if e != nil {
		return e
	}
	defer os.Remove(out.Name())
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	ce := out.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(out.Name(), target); e != nil {
		return e
	}
	return syncDirectory(filepath.Dir(target))
}
