package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (a *App) manageSubscriptions(ctx context.Context, form url.Values) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	action := form.Get("action")
	before := a.store.snapshot()
	if e := a.store.change(func(d Object) error {
		for _, v := range list(d["subscriptions"]) {
			sub := obj(v)
			key := "sub_url_" + str(sub["id"])
			if form.Has(key) {
				sub["url"] = strings.TrimSpace(form.Get(key))
				sub["use_hwid"] = form.Has("sub_hwid_" + str(sub["id"]))
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
			d["subscriptions"] = append(subs, Object{"id": id + 1, "name": fmt.Sprintf("Подписка #%d", id+1), "url": "", "use_hwid": false, "color": "#1971c2", "servers": []any{}})
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
