package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func trimBackupDirectory(dir string, keep int) error {
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	items := []os.FileInfo{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		items = append(items, info)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ModTime().Equal(items[j].ModTime()) {
			return items[i].Name() > items[j].Name()
		}
		return items[i].ModTime().After(items[j].ModTime())
	})
	for i := keep; i < len(items); i++ {
		if e = os.Remove(filepath.Join(dir, items[i].Name())); e != nil {
			return e
		}
	}
	return nil
}
