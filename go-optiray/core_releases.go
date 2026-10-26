package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxGitHubResponse = 8 << 20

var coreVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?$`)

func installedCoreVersion(ctx context.Context, path string) string {
	if path == "" {
		return ""
	}
	if _, e := os.Stat(path); e != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, path, "version").CombinedOutput()
	if e != nil {
		return "Неизвестно"
	}
	v := regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`).FindString(string(b))
	return first(v, "Установлено")
}
func fetchGitHubJSON(ctx context.Context, endpoint string) (any, error) {
	return fetchCoreGitHubJSON(ctx, "XTLS/Xray-core", endpoint)
}
func fetchCoreGitHubJSON(ctx context.Context, repository, endpoint string) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/"+repository+"/"+endpoint, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "OptiRay/"+version)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, fmt.Errorf("GitHub request failed: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, maxGitHubResponse+1))
	if e != nil {
		return nil, fmt.Errorf("GitHub response read failed: %w", e)
	}
	if len(b) > maxGitHubResponse {
		return nil, fmt.Errorf("GitHub response exceeds 8 MiB")
	}
	var value any
	e = json.Unmarshal(b, &value)
	return value, e
}
func releaseItem(v any) Object { return coreReleaseItem(v, "v") }
func coreReleaseItem(v any, prefix string) Object {
	r := obj(v)
	raw := str(r["tag_name"])
	if !strings.HasPrefix(raw, prefix) {
		return nil
	}
	tag := strings.TrimPrefix(raw, prefix)
	if boolean(r["draft"]) || !coreVersionPattern.MatchString(tag) {
		return nil
	}
	return Object{"version": tag, "is_pre": boolean(r["prerelease"])}
}
func (s *Store) xrayReleases(ctx context.Context, force bool) Object {
	return s.coreReleases(ctx, force, "XTLS/Xray-core", "xray", "v")
}
func (s *Store) hysteriaReleases(ctx context.Context, force bool) Object {
	return s.coreReleases(ctx, force, "apernet/hysteria", "hysteria", "app/v")
}
func (s *Store) coreReleases(ctx context.Context, force bool, repository, kind, prefix string) Object {
	s.releaseMu.Lock()
	defer s.releaseMu.Unlock()
	path := filepath.Join(s.dir, kind+"_releases_cache.json")
	cache, _ := readObject(path)
	valid := len(list(cache["recent"])) > 0 && str(obj(cache["stable"])["version"]) != ""
	source, message := "cache", ""
	if !valid {
		source = "unavailable"

	}
	// Opening settings reads the cache; only scheduled or manual checks reach GitHub.
	if force {
		source = "github"
		message = ""
		recent, e := fetchCoreGitHubJSON(ctx, repository, "releases?per_page=30")
		var stable any
		if e == nil {
			stable, e = fetchCoreGitHubJSON(ctx, repository, "releases/latest")
		}
		items := []any{}
		seen := map[string]bool{}
		for _, v := range list(recent) {
			item := coreReleaseItem(v, prefix)
			if item != nil && !seen[str(item["version"])] {
				items = append(items, item)
				seen[str(item["version"])] = true
			}
		}
		stableItem := coreReleaseItem(stable, prefix)
		if e == nil && len(items) > 0 && stableItem != nil && !boolean(stableItem["is_pre"]) {
			cache = Object{"recent": items, "stable": stableItem, "fetched_at": time.Now().Unix()}
			_ = writeObject(path, cache)
			valid = true
		} else {
			source = "unavailable"
			message = "Не удалось загрузить версии с GitHub"
			if e != nil {
				message += ": " + e.Error()
			}
			if valid {
				source = "cache"
				message += ". Показан сохранённый список"
			}
		}
	}
	available := []any{}
	if valid {
		stable := clone(cache["stable"])
		stable["is_stable"] = true
		for _, v := range list(cache["recent"]) {
			if str(obj(v)["version"]) != str(stable["version"]) && len(available) < 8 {
				available = append(available, v)
			}
		}
		available = append(available, stable)
	}
	return Object{"available": available, "versions_source": source, "versions_error": message}
}
