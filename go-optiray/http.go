package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type App struct {
	uiDir                 string
	store                 *Store
	core                  *CoreManager
	keenetic              *Keenetic
	routing               *RoutingProcess
	connectMu             sync.Mutex
	lastPowerSwitch       time.Time
	pingOnce              sync.Once
	pingMu                sync.Mutex
	tcpActive, realActive int
	pingChanged           chan struct{}
}

func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth, e := a.store.auth()
	if e != nil {
		http.Error(w, "Authentication settings unavailable", http.StatusServiceUnavailable)
		return
	}
	if e == nil && boolean(auth["enabled"]) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(str(auth["user"]))) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(str(auth["pass"]))) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="OptiRay", charset="UTF-8"`)
			http.Error(w, "Unauthorized", 401)
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	if a.routingAPI(w, r) {
		return
	}
	if a.routingWorkbook(w, r) {
		return
	}
	var err error
	switch {
	case r.Method == "POST" && (r.URL.Path == "/api/xray_action" || r.URL.Path == "/api/hysteria_action"):
		err = r.ParseForm()
		if err != nil {
			break
		}
		kind := "xray"
		if r.URL.Path == "/api/hysteria_action" {
			kind = "hysteria2"
		}
		err = a.coreAction(r.Context(), kind, r.Form.Get("action"), r.Form.Get("version"))
		status := 200
		message := "Ядро обновлено"
		if err != nil {
			status = 400
			message = err.Error()
		}
		jsonReply(w, status, Object{"ok": err == nil, "message": message})
		return
	case r.Method == "GET" && (r.URL.Path == "/api/xray_info" || r.URL.Path == "/api/hysteria_info"):
		path := filepath.Join(a.store.dir, "bin", "xray")
		hysteria := r.URL.Path == "/api/hysteria_info"
		if hysteria {
			path = filepath.Join(a.store.dir, "bin", "hysteria")
		}
		if a.core != nil {
			path = a.core.bin
			if hysteria {
				path = a.core.hysteriaBin
			}
		}
		installed := installedCoreVersion(r.Context(), path)
		info := Object{"installed": installed != "", "version": installed}
		if hysteria {
			info["available"] = []any{Object{"version": "2.6.2"}}
		} else {
			for k, v := range a.store.xrayReleases(r.Context(), r.URL.Query().Get("refresh") == "1") {
				info[k] = v
			}
		}
		jsonReply(w, 200, info)
		return
	case r.Method == "POST" && r.URL.Path == "/api/optiray_enabled":
		payload := Object{}
		err = json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			enabled, ok := payload["enabled"].(bool)
			if !ok {
				err = fmt.Errorf("enabled must be boolean")
			} else {
				err = a.setOptirayEnabled(r.Context(), enabled)
			}
		}
		if err == nil {
			jsonReply(w, 200, Object{"ok": true, "enabled": optirayEnabled(a.store.snapshot())})
			return
		}
	case r.Method == "GET" && r.URL.Path == "/api/optirout_config":
		cfg, e := a.store.loadOptirout()
		if e != nil {
			jsonReply(w, 500, Object{"error": e.Error()})
			return
		}
		jsonReply(w, 200, Object{"ok": true, "config": cfg, "save_backups": boolean(a.store.snapshot()["optirout_save_backups"]), "path": filepath.Join(a.store.dir, "routing", "optirout.json")})
		return
	case r.Method == "GET" && r.URL.Path == "/api/optirout_backups":
		items, e := a.store.optiroutBackups()
		if e != nil {
			jsonReply(w, 500, Object{"error": e.Error()})
			return
		}
		jsonReply(w, 200, items)
		return
	case r.Method == "GET" && r.URL.Path == "/api/optirout_status":
		if a.routing != nil {
			jsonReply(w, 200, a.routing.Status())
			return
		}
		jsonReply(w, 200, Object{"running": false, "pid": nil, "bin_name": nil, "config_path": filepath.Join(a.store.dir, "routing", "optirout.json"), "config_format": "json", "init_path": nil, "bin_path": nil, "supported": false, "message": "Управление процессом OptiRout отключено"})
		return
	case r.Method == "POST" && r.URL.Path == "/api/optirout_config":
		payload := Object{}
		err = json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			break
		}
		if payload["config"] == nil {
			err = fmt.Errorf("config required")
			break
		}
		restart := true
		if v, exists := payload["restart"]; exists {
			restart = boolean(v)
		}
		if restart && str(a.store.snapshot()["routing_engine"]) != "xray" && a.routing == nil {
			jsonReply(w, 503, Object{"error": "Управление процессом OptiRout отключено; конфигурация не изменена"})
			return
		}
		err = a.saveRouting(r.Context(), obj(payload["config"]), restart)
		if err == nil {
			restarted := restart && a.routingWanted(a.store.snapshot())
			jsonReply(w, 200, Object{"ok": true, "restarted": restarted})
			return
		}
	case r.Method == "POST" && r.URL.Path == "/api/optirout_restart":
		if a.routing == nil {
			jsonReply(w, 503, Object{"error": "Управление OptiRout отключено"})
			return
		}
		a.connectMu.Lock()
		if a.routingWanted(a.store.snapshot()) {
			err = a.routing.Restart(r.Context())
		} else {
			err = fmt.Errorf("OptiRout выключен")
		}
		a.connectMu.Unlock()
		if err == nil {
			jsonReply(w, 200, Object{"ok": true})
			return
		}
	case r.Method == "POST" && r.URL.Path == "/api/auto_select":
		if a.core == nil {
			jsonReply(w, 503, Object{"error": "Запуск ядра отключён для этой тестовой установки"})
			return
		}
		err = r.ParseForm()
		if err != nil {
			break
		}
		subID, e := strconv.Atoi(r.Form.Get("sub_id"))
		if e != nil || subID < 1 {
			jsonReply(w, 400, Object{"error": "sub_id required"})
			return
		}
		result, e := a.autoSelect(r.Context(), subID)
		if e != nil {
			result["error"] = e.Error()
			jsonReply(w, 400, result)
			return
		}
		jsonReply(w, 200, result)
		return
	case r.Method == "GET" && r.URL.Path == "/api/ping":
		d := a.store.snapshot()
		q := r.URL.Query()
		kind := first(q.Get("type"), "real")
		if kind != "tcp" && kind != "real" {
			jsonReply(w, 400, Object{"error": "invalid ping type"})
			return
		}
		seconds, _ := strconv.ParseFloat(first(q.Get("timeout"), str(d["ping_timeout"])), 64)
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0.5 || seconds > 30 {
			seconds = 3
		}
		server, _ := findServer(d, q.Get("sid"), q.Get("name"))
		latency := -1
		if server != nil {
			latency = a.ping(r.Context(), d, server, kind, time.Duration(seconds*float64(time.Second)))
		}
		jsonReply(w, 200, Object{"latency": latency, "type": kind})
		return
	case r.Method == "GET" && (r.URL.Path == "/favicon.ico" || r.URL.Path == "/favicon.svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><circle cx="32" cy="32" r="30" fill="#f0f0f0" stroke="#141414" stroke-width="3"/><path d="M17 27h24m-5-7 7 7-7 7M47 37H23m5-7-7 7 7 7" fill="none" stroke="#141414" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
		return
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/flag/"):
		code := strings.TrimPrefix(r.URL.Path, "/api/flag/")
		if !regexp.MustCompile(`^[a-z]{2}$`).MatchString(code) {
			http.NotFound(w, r)
			return
		}
		content, e := assets.ReadFile("assets/flags/" + code + ".svg")
		if e != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write(content)
		return
	case r.Method == "POST" && r.URL.Path == "/api/restore":
		payload := Object{}
		err = json.NewDecoder(r.Body).Decode(&payload)
		if err == nil {
			err = a.restoreState(r.Context(), payload)
		}
	case r.Method == "GET" && (r.URL.Path == "/assets/style.css" || r.URL.Path == "/assets/app.js"):
		name := strings.TrimPrefix(r.URL.Path, "/assets/")
		content, e := a.uiAsset(name)
		if e != nil {
			http.Error(w, "UI asset unavailable", 500)
			return
		}
		if name == "style.css" {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(content)
		return
	case r.Method == "POST" && r.URL.Path == "/select":
		if a.core == nil {
			jsonReply(w, 503, Object{"error": "Запуск ядра отключён для этой тестовой установки"})
			return
		}
		err = r.ParseForm()
		if err != nil {
			break
		}
		result, e := a.connectSelection(r.Context(), r.Form.Get("sid"), r.Form.Get("server_name"))
		if e != nil {
			err = e
			break
		}
		jsonReply(w, 200, result)
		return
	case r.Method == "POST" && r.URL.Path == "/api/save_hwid":
		err = r.ParseForm()
		if err == nil {
			value := strings.TrimSpace(r.Form.Get("hwid"))
			if value == "" {
				err = fmt.Errorf("empty hwid")
			} else {
				err = a.store.change(func(d Object) error { d["hwid"] = value; return nil })
			}
		}
	case r.Method == "POST" && r.URL.Path == "/manage":
		err = r.ParseForm()
		if err != nil {
			break
		}
		action := r.Form.Get("action")
		if action == "save_settings" {
			err = a.saveSettings(r.Context(), r.Form)
		} else {
			err = a.manageSubscriptions(r.Context(), r.Form)
		}
		if err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	case r.Method == "GET" && r.URL.Path == "/":
		a.render(w)
		return
	case r.Method == "GET" && r.URL.Path == "/api/health":
		d := a.store.snapshot()
		coreReady := !optirayEnabled(d) || str(d["selected"]) == "" || a.core != nil && a.core.Running()
		routingReady := !a.routingWanted(d) || a.routing != nil && boolean(a.routing.Status()["running"])
		jsonReply(w, 200, Object{"ok": true, "version": version, "runtime": "go", "release_ready": false, "ready": coreReady && routingReady, "core_ready": coreReady, "routing_ready": routingReady})
		return
	case r.Method == "GET" && r.URL.Path == "/api/xray_routing_config":
		d := a.store.snapshot()
		jsonReply(w, 200, Object{"ok": true, "routing": d["routing"], "save_backups": d["xray_save_backups"]})
		return
	case r.Method == "GET" && r.URL.Path == "/api/backup":
		settings := a.store.snapshot()
		q := r.URL.Query()
		optirout := boolean(settings["optirout_save_backups"])
		xray := boolean(settings["backup_include_xray"])
		if q.Has("optirout") {
			optirout = q.Get("optirout") == "1"
		}
		if q.Has("xray") {
			xray = q.Get("xray") == "1"
		}
		d, name, e := a.store.backup(optirout, xray)
		if e != nil {
			jsonReply(w, 500, Object{"ok": false, "error": e.Error()})
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
		jsonReply(w, 200, d)
		return
	case r.Method == "POST" && r.URL.Path == "/api/toggle_favorite":
		err = r.ParseForm()
		fav := false
		if err == nil {
			err = a.store.change(func(d Object) error {
				s, _ := findServer(d, r.Form.Get("sid"), r.Form.Get("server_name"))
				if s == nil {
					return fmt.Errorf("server not found")
				}
				sid := str(s["sid"])
				items := []any{}
				seen := map[string]bool{}
				exists := false
				for _, v := range list(d["favorites"]) {
					id := str(v)
					if id == sid {
						exists = true
						continue
					}
					if !seen[id] {
						items = append(items, id)
						seen[id] = true
					}
				}
				if !exists {
					items = append(items, sid)
					fav = true
				}
				d["favorites"] = items
				return nil
			})
		}
		if err == nil {
			jsonReply(w, 200, Object{"status": "ok", "favorite": fav})
			return
		}
	case r.Method == "POST" && r.URL.Path == "/api/reorder_subs":
		body := Object{}
		err = json.NewDecoder(r.Body).Decode(&body)
		if err == nil {
			err = a.store.change(func(d Object) error {
				old := list(d["subscriptions"])
				next := []any{}
				seen := map[int]bool{}
				for _, v := range list(body["order"]) {
					id := integer(v)
					if seen[id] {
						continue
					}
					for _, sub := range old {
						if integer(obj(sub)["id"]) == id {
							next = append(next, sub)
							seen[id] = true
							break
						}
					}
				}
				for _, sub := range old {
					if !seen[integer(obj(sub)["id"])] {
						next = append(next, sub)
					}
				}
				d["subscriptions"] = next
				return nil
			})
		}
	case r.Method == "POST" && r.URL.Path == "/api/save_sub_color":
		err = r.ParseForm()
		if err == nil {
			err = a.store.change(func(d Object) error {
				color := r.Form.Get("color")
				if !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(color) {
					return fmt.Errorf("invalid color")
				}
				id := r.Form.Get("sub_id")
				for _, v := range list(d["subscriptions"]) {
					s := obj(v)
					if str(s["id"]) == id {
						s["color"] = color
						return nil
					}
				}
				return fmt.Errorf("subscription not found")
			})
		}
	default:
		jsonReply(w, http.StatusNotFound, Object{"ok": false, "error": "Unknown endpoint"})
		return
	}
	if err != nil {
		jsonReply(w, 400, Object{"ok": false, "error": err.Error()})
		return
	}
	jsonReply(w, 200, Object{"status": "ok", "ok": true})
}
func backupName(connection, kind, extension string, now time.Time) string {
	connection = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f\x7f]`).ReplaceAllString(connection, "_")
	connection = strings.Trim(connection, " .")
	if connection == "" {
		connection = "OptiRay"
	}
	return connection + "-" + kind + "_" + now.Format("02-01-2006_1504") + "." + extension
}
func (a *App) render(w http.ResponseWriter) {
	d := a.store.snapshot()
	page, e := a.uiAsset("page.html")
	if e != nil {
		http.Error(w, "UI template unavailable", 500)
		return
	}
	s := string(page)
	rows, chips := renderSubscriptionInputs(d)
	s = strings.ReplaceAll(s, "__SUB_INPUTS__", rows)
	s = strings.ReplaceAll(s, "__SUB_NAMES__", chips)
	s = strings.ReplaceAll(s, "__CARDS_HTML__", renderServerCards(d))
	powerLabel := "OptiRay выкл."
	if optirayEnabled(d) {
		powerLabel = "OptiRay вкл."
	}
	values := map[string]string{"__OPTIRAY_POWER_LABEL__": powerLabel, "__APP_VER__": version, "__PROXY_NAME__": str(d["proxy_name"]), "__PROXY_HOST__": str(d["proxy_host"]), "__PROXY_PORT__": str(d["proxy_port"]), "__HWID_VAL__": str(d["hwid"]), "__TEST_URL__": str(d["test_url"]), "__TEST_BUTTON_URL__": str(d["test_button_url"]), "__ROUTING_URL__": str(d["routing_url"]), "__PING_TIMEOUT__": str(d["ping_timeout"]), "__PING_TCP_CONC__": str(d["ping_tcp_concurrency"]), "__PING_REAL_CONC__": str(d["ping_real_concurrency"]), "__PING_SAMPLE_DELAY_MS__": str(d["ping_sample_delay_ms"]), "__SUBS_COUNT__": fmt.Sprint(len(list(d["subscriptions"]))), "__ACTIVE_SUB_COLOR__": "#6c757d", "__ACTIVE_SUB_NAME__": "Не выбрано", "__ACTIVE_DOT_CLASS__": "disconnected", "__ACTIVE_CORE_HIDDEN__": "display:none;", "__ROUTING_ENGINE__": str(d["routing_engine"])}
	if server, sub := findServer(d, str(d["selected"]), ""); server != nil {
		values["__ACTIVE_SUB_COLOR__"] = colorOf(sub)
		values["__ACTIVE_SUB_NAME__"] = str(sub["name"])
		values["__ACTIVE_SERVER_NAME__"] = str(server["name"])
		s = strings.ReplaceAll(s, "__ACTIVE_SERVER_FLAG__", serverFlag(str(server["name"])))
		if a.core != nil && a.core.Running() {
			values["__ACTIVE_DOT_CLASS__"] = "connected"
			values["__ACTIVE_CORE__"] = "Xray"
			if str(server["protocol"]) == "hysteria" || str(server["protocol"]) == "hysteria2" {
				values["__ACTIVE_CORE__"] = "Hysteria"
				values["__ACTIVE_CORE_CLASS__"] = "core-hysteria"
			}
			if values["__ACTIVE_CORE_CLASS__"] == "" {
				values["__ACTIVE_CORE_CLASS__"] = "core-xray"
			}
			values["__ACTIVE_CORE_HIDDEN__"] = ""
		}
	}
	for k, v := range values {
		s = strings.ReplaceAll(s, k, html.EscapeString(v))
	}
	auth, _ := a.store.auth()
	s = strings.ReplaceAll(s, "__AUTH_USER__", esc(auth["user"]))
	routes := obj(d["routing"])
	for _, key := range []string{"block", "direct", "proxy"} {
		s = strings.ReplaceAll(s, "__ROUTING_"+strings.ToUpper(key)+"_RULES__", esc(routes[key+"_rules"]))
	}
	s = strings.ReplaceAll(s, "__XRAY_AVAILABLE__", strconv.FormatBool(a.xrayInstalled()))
 stateTokens := map[string]bool{"__OPTIRAY_ENABLED__": optirayEnabled(d), "__AUTH_ENABLED__": boolean(auth["enabled"]), "__ASRC_ALL__": str(d["auto_select_source"]) != "favorites", "__ASRC_FAVS__": str(d["auto_select_source"]) == "favorites", "__ROUTING_ENABLED__": boolean(routes["enabled"]), "__ROUTING_MODE_DIRECT__": boolean(routes["direct_default"]), "__ROUTING_MODE_PROXY__": !boolean(routes["direct_default"])}
	for token, on := range stateTokens {
		value := ""
		if on {
			value = "checked"
		}
		s = strings.ReplaceAll(s, token, value)
	}
	external := "0"
	if boolean(d["routing_use_external"]) {
		external = "1"
	}
	s = strings.ReplaceAll(s, "__ROUTING_USE_EXTERNAL_VAL__", external)
	hidden := ""
	if !boolean(d["show_ping_all"]) {
		hidden = "display:none;"
	}
	s = strings.ReplaceAll(s, "__GLOBAL_PING_HIDDEN__", hidden)
	floating := ""
	if boolean(d["show_test_float"]) && validHTTPURL(str(d["test_button_url"])) {
		floating = `<a id="test_float_btn" class="btn-test-float" href="` + esc(d["test_button_url"]) + `" target="_blank" rel="noopener noreferrer">` + openIcon + `</a>`
	}
	s = strings.ReplaceAll(s, "__TEST_FLOAT_BTN__", floating)
	checked := map[string]string{"__SHOW_BR0__": "show_br0", "__LOGGING_CHECKED__": "enable_logging", "__SHOW_PING_ALL__": "show_ping_all", "__SHOW_TEST_BUTTON__": "show_test_button", "__SHOW_TEST_FLOAT__": "show_test_float", "__UPDATE_FORCE_RESTART__": "update_force_restart", "__OPTIROUT_SAVE_BACKUPS__": "optirout_save_backups", "__BACKUP_INCLUDE_XRAY__": "backup_include_xray"}
	for token, key := range checked {
		v := ""
		if boolean(d[key]) {
			v = "checked"
		}
		s = strings.ReplaceAll(s, token, v)
	}
	profile := str(d["subscription_client_profile"])
	for _, key := range []string{"happ", "incy", "optiray"} {
		v := ""
		if key == profile {
			v = "selected"
		}
		s = strings.ReplaceAll(s, "__UA_"+strings.ToUpper(key)+"_SELECTED__", v)
	}
	s = strings.ReplaceAll(s, "__OPTIROUT_SAVE_BACKUPS_JS__", fmt.Sprint(boolean(d["optirout_save_backups"])))
	groups, _ := json.Marshal(obj(d["routing"])["groups_ui"])
	s = strings.ReplaceAll(s, "__ROUTING_GROUPS_JSON__", html.EscapeString(string(groups)))
	s = regexp.MustCompile(`__[A-Z][A-Z0-9_]*__`).ReplaceAllString(s, "")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(s))
}
