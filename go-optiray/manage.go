package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *App) manageSubscriptions(ctx context.Context, form url.Values) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	action := form.Get("action")
	if (action == "add" || action == "update") && !a.xrayInstalled() {
		return fmt.Errorf("Установите ядро Xray в настройках")
	}
	if action == "delete_config" {
		index, err := strconv.Atoi(form.Get("config_index"))
		if err != nil {
			return err
		}
		return a.store.change(func(d Object) error {
			for _, v := range list(d["subscriptions"]) {
				sub := obj(v)
				if str(sub["id"]) != form.Get("sub_id") || !boolean(sub["standalone_group"]) {
					continue
				}
				servers, links := list(sub["servers"]), list(sub["config_links"])
				if index < 0 || index >= len(servers) || index >= len(links) {
					return fmt.Errorf("Конфигурация не найдена")
				}
				sid := str(obj(servers[index])["sid"])
				if str(d["selected"]) == sid {
					return fmt.Errorf("Сначала отключите удаляемую конфигурацию")
				}
				sub["servers"] = append(servers[:index:index], servers[index+1:]...)
				sub["config_links"] = append(links[:index:index], links[index+1:]...)
				favs := []any{}
				for _, fav := range list(d["favorites"]) {
					if str(fav) != sid {
						favs = append(favs, fav)
					}
				}
				d["favorites"] = favs
				pruneEmptyConfigurationGroups(d)
				return nil
			}
			return fmt.Errorf("Группа не найдена")
		})
	}
	if action == "edit_config" {
		if !a.xrayInstalled() {
			return fmt.Errorf("Установите ядро Xray в настройках")
		}
		server, err := parseLink(strings.TrimSpace(form.Get("config_link")))
		if err != nil {
			return fmt.Errorf("Некорректная конфигурация: %w", err)
		}
		server["sid"] = serverID(server)
		index, err := strconv.Atoi(form.Get("config_index"))
		if err != nil {
			return err
		}
		return a.store.change(func(d Object) error {
			for _, v := range list(d["subscriptions"]) {
				sub := obj(v)
				if str(sub["id"]) != form.Get("sub_id") || !boolean(sub["standalone_group"]) {
					continue
				}
				servers, links := list(sub["servers"]), list(sub["config_links"])
				if index < 0 || index >= len(servers) || index >= len(links) {
					return fmt.Errorf("Конфигурация не найдена")
				}
				oldID := str(obj(servers[index])["sid"])
				if str(d["selected"]) == oldID {
					return fmt.Errorf("Сначала отключите редактируемую конфигурацию")
				}
				for i, fav := range list(d["favorites"]) {
					if str(fav) == oldID {
						list(d["favorites"])[i] = server["sid"]
					}
				}
				servers[index] = server
				links[index] = strings.TrimSpace(form.Get("config_link"))
				return nil
			}
			return fmt.Errorf("Группа не найдена")
		})
	}
	before := a.store.snapshot()
	if e := a.store.change(func(d Object) error {
		for _, v := range list(d["subscriptions"]) {
			sub := obj(v)
			key := "sub_url_" + str(sub["id"])
			if form.Has(key) && !boolean(sub["standalone_group"]) {
				sub["url"] = strings.TrimSpace(form.Get(key))
				sub["use_hwid"] = form.Has("sub_hwid_" + str(sub["id"]))
			}
		}
		if action == "add" || action == "update" {
			if err := collectStandaloneConfigs(d); err != nil {
				return err
			}
		}
		if action == "add" {
			subs := list(d["subscriptions"])
			id := 0
			for _, v := range subs {
				if n := integer(obj(v)["id"]); n > id {
					id = n
				}
			}
			d["subscriptions"] = append(subs, Object{"id": id + 1, "name": fmt.Sprintf("Новое подкл. #%d", id+1), "url": "", "use_hwid": false, "color": []string{"#2b8a3e", "#1971c2", "#d9480f", "#862e9c", "#0c8599", "#e67700", "#495057"}[rand.IntN(7)], "servers": []any{}})
			return nil
		}
		if strings.HasPrefix(action, "del_") {
			id, e := strconv.Atoi(strings.TrimPrefix(action, "del_"))
			if e != nil {
				return e
			}
			subs := []any{}
			removed := false
			for _, v := range list(d["subscriptions"]) {
				sub := obj(v)
				if integer(sub["id"]) == id {
					removed = true
					for _, srv := range list(sub["servers"]) {
						if str(obj(srv)["sid"]) == str(d["selected"]) {
							d["selected"] = ""
						}
					}
				} else {
					subs = append(subs, v)
				}
			}
			if !removed {
				return fmt.Errorf("subscription not found")
			}
			d["subscriptions"] = subs
			return nil
		}
		if action != "update" {
			return fmt.Errorf("unknown action")
		}
		return nil
	}); e != nil {
		return e
	}
	if action == "update" {
		e := a.store.refreshApply(ctx, fetchSubscription, func(next Object) error {
			srv, _ := findServer(next, str(next["selected"]), "")
			if srv == nil {
				if a.core != nil {
					a.core.Stop()
				}
				return nil
			}
			if a.core == nil {
				return fmt.Errorf("active core management is disabled")
			}
			return a.core.ApplyServer(srv, next, integer(next["proxy_port"]))
		})
		if e != nil {
			if rollback := a.rollbackRuntime(before, nil); rollback != nil {
				return fmt.Errorf("refresh failed: %v; core rollback failed: %v", e, rollback)
			}
			return e
		}
	}
	if strings.HasPrefix(action, "del_") && str(a.store.snapshot()["selected"]) == "" && a.core != nil {
		a.core.Stop()
	}
	return nil
}

func (a *App) xrayInstalled() bool {
	path := filepath.Join(a.store.dir, "bin", "xray")
	if a.core != nil {
		path = a.core.bin
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

// Consolidate manually entered URI sources without touching HTTP subscriptions.
func collectStandaloneConfigs(d Object) error {
	if err := expandJSONConfigurations(d); err != nil {
		return err
	}
	subs := list(d["subscriptions"])
	var group Object
	for _, v := range subs {
		if boolean(obj(v)["standalone_group"]) {
			group = obj(v)
			break
		}
	}
	out := []any{}
	for _, v := range subs {
		sub := obj(v)
		raw := strings.TrimSpace(str(sub["url"]))
		uri, err := url.Parse(raw)
		standalone := err == nil && uri.Scheme != "" && uri.Scheme != "http" && uri.Scheme != "https"
		if !standalone || boolean(sub["standalone_group"]) {
			out = append(out, sub)
			continue
		}
		server, err := parseLink(raw)
		if err != nil {
			return fmt.Errorf("Некорректная конфигурация: %w", err)
		}
		if group == nil {
			group = Object{"id": sub["id"], "name": "Конфигурации", "url": "", "color": "#101010", "standalone_group": true, "servers": []any{}, "config_links": []any{}}
			out = append(out, group)
		}
		server["sid"] = serverID(server)
		uniqueServerIDs(list(group["servers"]))
		duplicate := false
		for _, old := range list(group["servers"]) {
			if str(obj(old)["sid"]) == str(server["sid"]) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			group["servers"] = append(list(group["servers"]), server)
			group["config_links"] = append(list(group["config_links"]), raw)
		}
	}
	if group != nil {
		group["name"] = "Конфигурации"
		group["color"] = "#101010"
		group["use_hwid"] = false
	}
	d["subscriptions"] = out
	return nil
}

func pruneEmptyConfigurationGroups(d Object) {
	kept := []any{}
	for _, entry := range list(d["subscriptions"]) {
		sub := obj(entry)
		servers, valid := sub["servers"].([]any)
		if boolean(sub["standalone_group"]) && valid && len(servers) == 0 {
			continue
		}
		kept = append(kept, entry)
	}
	d["subscriptions"] = kept
}
