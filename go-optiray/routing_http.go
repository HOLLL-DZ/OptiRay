package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (a *App) routingAPI(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if path == "/api/routing_power" {
		payload := Object{}
		if r.Method != "POST" {
			jsonReply(w, 405, Object{"ok": false, "error": "POST required"})
			return true
		}
		err := json.NewDecoder(r.Body).Decode(&payload)
		enabled, valid := payload["enabled"].(bool)
		if err == nil && !valid {
			err = fmt.Errorf("enabled must be boolean")
		}
		if err == nil {
			err = a.setRoutingPower(r.Context(), str(payload["engine"]), enabled)
		}
		if err != nil {
			jsonReply(w, 400, Object{"ok": false, "error": err.Error()})
		} else {
			jsonReply(w, 200, Object{"ok": true, "enabled": enabled})
		}
		return true
	}
	handled := path == "/api/optirout_license" || path == "/api/optirout_interfaces" || path == "/api/routing_rules_mode" || path == "/api/xray_backup_settings" || path == "/api/optirout_backup_settings" || path == "/api/xray_reset" || path == "/api/optirout_reset" || path == "/api/optirout_restore_backup" || path == "/api/reset_defaults"
	if !handled {
		return false
	}
	var e error
	result := Object{"ok": true}
	payload := Object{}
	if r.Method == "POST" {
		if r.ContentLength != 0 {
			e = json.NewDecoder(r.Body).Decode(&payload)
		}
	} else if r.Method != "GET" {
		jsonReply(w, 405, Object{"error": "method not allowed"})
		return true
	}
	if e == nil {
		switch path {
		case "/api/optirout_license":
			if r.Method != "GET" {
				e = fmt.Errorf("GET required")
				break
			}
			parts := []string{}
			for _, name := range []string{"NOTICE.optirout", "AUTHORS.optirout", "LICENSE.optirout"} {
				b, err := assets.ReadFile("assets/" + name)
				if err != nil {
					e = err
					break
				}
				parts = append(parts, string(b))
			}
			if e == nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = w.Write([]byte(strings.Join(parts, "\n\n")))
				return true
			}
		case "/api/optirout_interfaces":
			if r.Method != "GET" {
				e = fmt.Errorf("GET required")
				break
			}
			var items []any
			items, e = a.interfaces(r.Context())
			result["interfaces"] = items
		case "/api/xray_backup_settings", "/api/optirout_backup_settings":
			value, ok := payload["enabled"].(bool)
			if !ok || r.Method != "POST" {
				e = fmt.Errorf("enabled must be boolean")
				break
			}
			key := "xray_save_backups"
			if path == "/api/optirout_backup_settings" {
				key = "optirout_save_backups"
			}
			e = a.store.change(func(d Object) error { d[key] = value; return nil })
			result["enabled"] = value
		case "/api/routing_rules_mode":
			if r.Method != "POST" {
				e = fmt.Errorf("POST required")
				break
			}
			var routing Object
			if payload["routing"] != nil {
				routing = obj(payload["routing"])
				if len(routing) == 0 {
					e = fmt.Errorf("invalid routing")
					break
				}
			}
			e = a.routingMode(r.Context(), str(payload["engine"]), routing)
			result["engine"] = payload["engine"]
		case "/api/xray_reset":
			if r.Method != "POST" {
				e = fmt.Errorf("POST required")
				break
			}
			d, err := defaultData()
			if err != nil {
				e = err
				break
			}
			e = a.routingMode(r.Context(), str(a.store.snapshot()["routing_engine"]), obj(d["routing"]))
		case "/api/optirout_reset":
			if r.Method != "POST" {
				e = fmt.Errorf("POST required")
				break
			}
			cfg, err := defaultOptirout()
			if err != nil {
				e = err
				break
			}
			e = a.saveRouting(r.Context(), cfg, true)
		case "/api/optirout_restore_backup":
			if r.Method != "POST" {
				e = fmt.Errorf("POST required")
				break
			}
			name := str(payload["name"])
			var cfg Object
			cfg, e = a.readRoutingBackup(name)
			if e == nil {
				e = a.saveRouting(r.Context(), cfg, true)
			}
			result["restored"] = name
		case "/api/reset_defaults":
			if r.Method != "POST" {
				e = fmt.Errorf("POST required")
				break
			}
			e = a.resetState(r.Context())
		}
	}
	if e != nil {
		jsonReply(w, 400, Object{"ok": false, "error": e.Error()})
	} else {
		jsonReply(w, 200, result)
	}
	return true
}
