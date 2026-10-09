package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Sources and backups reside on the same filesystem as targets.
func replaceCoreFiles(root, stage string, names []string) (func() error, error) {
	sort.Strings(names)
	changed := []string{}
	had := map[string]bool{}
	undo := func() error {
		for i := len(changed) - 1; i >= 0; i-- {
			name := changed[i]
			target := filepath.Join(root, name)
			if e := os.Remove(target); e != nil && !os.IsNotExist(e) {
				return e
			}
			if had[name] {
				if e := os.Rename(filepath.Join(stage, "old-"+name), target); e != nil {
					return e
				}
			}
		}
		return nil
	}
	for _, name := range names {
		if name != "xray" && name != "hysteria" && name != "geoip.dat" && name != "geosite.dat" {
			return nil, fmt.Errorf("invalid core file")
		}
		target := filepath.Join(root, name)
		if _, e := os.Stat(target); e == nil {
			if e = os.Rename(target, filepath.Join(stage, "old-"+name)); e != nil {
				if rollback := undo(); rollback != nil {
					return nil, fmt.Errorf("replace failed: %v; rollback: %v", e, rollback)
				}
				return nil, e
			}
			had[name] = true
		} else if !os.IsNotExist(e) {
			_ = undo()
			return nil, e
		}
		changed = append(changed, name)
		if e := os.Rename(filepath.Join(stage, name), target); e != nil {
			if rollback := undo(); rollback != nil {
				return nil, fmt.Errorf("replace failed: %v; rollback: %v", e, rollback)
			}
			return nil, e
		}
	}
	return undo, nil
}
