package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type coreVersionStamp struct {
	stat          os.FileInfo
	path, version string
}
type coreIndicatorState struct {
	mu       sync.Mutex
	versions map[string]coreVersionStamp
}

func newerCoreVersion(candidate, current string) bool {
	parse := func(value string) ([3]int, bool) {
		var result [3]int
		value = strings.TrimPrefix(value, "v")
		base := strings.SplitN(strings.SplitN(value, "+", 2)[0], "-", 2)[0]
		parts := strings.Split(base, ".")
		if len(parts) != 3 {
			return result, false
		}
		for i, part := range parts {
			v, err := strconv.Atoi(part)
			if err != nil || v < 0 {
				return result, false
			}
			result[i] = v
		}
		return result, true
	}
	next, ok := parse(candidate)
	if !ok {
		return false
	}
	old, ok := parse(current)
	if !ok {
		return false
	}
	for i := range next {
		if next[i] != old[i] {
			return next[i] > old[i]
		}
	}
	return !strings.Contains(candidate, "-") && strings.Contains(current, "-")
}
func (a *App) indicatorInstalledVersion(ctx context.Context, kind, path string) string {
	a.coreIndicators.mu.Lock()
	defer a.coreIndicators.mu.Unlock()
	if a.coreIndicators.versions == nil {
		a.coreIndicators.versions = map[string]coreVersionStamp{}
	}
	stat, err := os.Stat(path)
	if err != nil {
		delete(a.coreIndicators.versions, kind)
		return ""
	}
	cached := a.coreIndicators.versions[kind]
	if cached.stat != nil && cached.path == path && os.SameFile(cached.stat, stat) && cached.stat.Size() == stat.Size() && cached.stat.ModTime().Equal(stat.ModTime()) {
		return cached.version
	}
	value := installedCoreVersion(ctx, path)
	a.coreIndicators.versions[kind] = coreVersionStamp{stat: stat, path: path, version: value}
	return value
}
func (a *App) coreUpdateStatus(ctx context.Context) Object {
	result := Object{"ok": true}
	anyUpdate := false
	for _, kind := range []string{"xray", "hysteria"} {
		path := filepath.Join(a.store.dir, "bin", kind)
		if a.core != nil {
			path = a.core.bin
			if kind == "hysteria" {
				path = a.core.hysteriaBin
			}
		}
		current := a.indicatorInstalledVersion(ctx, kind, path)
		var releases Object
		if kind == "xray" {
			releases = a.store.xrayReleases(ctx, false)
		} else {
			releases = a.store.hysteriaReleases(ctx, false)
		}
		latest := ""
		for _, value := range list(releases["available"]) {
			item := obj(value)
			if boolean(item["is_stable"]) {
				latest = str(item["version"])
				break
			}
		}
		available := current != "" && newerCoreVersion(latest, current)
		anyUpdate = anyUpdate || available
		result[kind] = Object{"installed": current != "", "current": current, "latest": latest, "update_available": available}
	}
	result["update_available"] = anyUpdate
	return result
}
