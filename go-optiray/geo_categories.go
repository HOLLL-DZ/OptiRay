package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type geoCategory struct {
	Code       string   `json:"code"`
	Count      int      `json:"count"`
	Attributes []string `json:"attributes,omitempty"`
}
type geoCatalogEntry struct {
	stat       os.FileInfo
	categories []geoCategory
	err        string
}
type geoCatalogState struct {
	mu    sync.Mutex
	files map[string]geoCatalogEntry
}

func parseGeoCategories(data []byte) ([]geoCategory, error) {
	result := []geoCategory{}
	seen := map[string]bool{}
	err := geoFields(data, func(field, wire uint64, value []byte, _ uint64) error {
		if field != 1 || wire != 2 {
			return fmt.Errorf("Некорректная геобаза")
		}
		item := geoCategory{}
		attrs := map[string]bool{}
		err := geoFields(value, func(field, wire uint64, value []byte, _ uint64) error {
			if field == 1 {
				if wire != 2 {
					return fmt.Errorf("Некорректный код категории")
				}
				item.Code = strings.ToLower(string(value))
			}
			if field == 2 && wire == 2 {
				item.Count++
				// Domain attributes are selectable suffixes, e.g. category@ads.
				return geoFields(value, func(field, wire uint64, value []byte, _ uint64) error {
					if field != 3 || wire != 2 {
						return nil
					}
					return geoFields(value, func(field, wire uint64, value []byte, _ uint64) error {
						if field == 1 && wire == 2 {
							attrs[strings.ToLower(string(value))] = true
						}
						return nil
					})
				})
			}
			return nil
		})
		if err != nil {
			return err
		}
		if item.Code == "" || seen[item.Code] {
			return fmt.Errorf("Некорректные категории геобазы")
		}
		seen[item.Code] = true
		for attr := range attrs {
			if attr != "" {
				item.Attributes = append(item.Attributes, attr)
			}
		}
		sort.Strings(item.Attributes)
		result = append(result, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Геобаза не содержит категорий")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result, nil
}
func (a *App) geoCategories(w http.ResponseWriter, r *http.Request) {
	a.geoCatalog.mu.Lock()
	defer a.geoCatalog.mu.Unlock()
	if a.geoCatalog.files == nil {
		a.geoCatalog.files = map[string]geoCatalogEntry{}
	}
	result := Object{"ok": true}
	for _, name := range geoNames {
		path := filepath.Join(a.store.dir, "bin", name)
		stat, err := os.Stat(path)
		item := Object{"exists": false, "categories": []geoCategory{}}
		if err != nil {
			if !os.IsNotExist(err) {
				item["error"] = "Не удалось прочитать " + name
			}
			result[strings.TrimSuffix(name, ".dat")] = item
			continue
		}
		item["exists"] = true
		cached := a.geoCatalog.files[name]
		if cached.stat == nil || !os.SameFile(stat, cached.stat) || stat.Size() != cached.stat.Size() || !stat.ModTime().Equal(cached.stat.ModTime()) {
			cached = geoCatalogEntry{stat: stat}
			if !stat.Mode().IsRegular() || stat.Size() > 128<<20 {
				cached.err = "Некорректный размер " + name
			} else {
				file, err := os.Open(path)
				if err == nil {
					var data []byte
					data, err = io.ReadAll(io.LimitReader(file, (128<<20)+1))
					file.Close()
					if err == nil {
						cached.categories, err = parseGeoCategories(data)
					}
				}
				if err != nil {
					cached.err = "Не удалось прочитать категории " + name
				}
			}
			a.geoCatalog.files[name] = cached
		}
		if cached.err != "" {
			item["error"] = cached.err
		} else {
			item["categories"] = cached.categories
		}
		result[strings.TrimSuffix(name, ".dat")] = item
	}
	jsonReply(w, 200, result)
}
