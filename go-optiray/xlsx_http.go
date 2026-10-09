package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *App) routingWorkbook(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if r.Method != "POST" || (path != "/api/routing_import" && path != "/api/routing_export" && path != "/api/optirout_import" && path != "/api/optirout_export") {
		return false
	}
	payload := Object{}
	if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
		jsonReply(w, 400, Object{"error": "invalid JSON"})
		return true
	}
	optirout := strings.HasPrefix(path, "/api/optirout_")
	if strings.HasSuffix(path, "_import") {
		b64 := str(payload["file_b64"])
		if strings.HasPrefix(b64, "data:") {
			parts := strings.SplitN(b64, ",", 2)
			if len(parts) == 2 {
				b64 = parts[1]
			}
		}
		blob, e := decode64(b64)
		if e != nil {
			jsonReply(w, 400, Object{"error": e.Error()})
			return true
		}
		rows, e := parseXLSX(blob)
		if e != nil {
			jsonReply(w, 400, Object{"error": e.Error()})
			return true
		}
		groups, e := groupsFromRows(rows, optirout)
		if e != nil {
			jsonReply(w, 400, Object{"error": e.Error()})
			return true
		}
		jsonReply(w, 200, Object{"ok": true, "groups": groups})
		return true
	}
	var groups any
	kind := "xray_rules"
	if optirout {
		kind = "optirout"
		cfg := obj(payload["config"])
		if payload["config"] == nil {
			var e error
			cfg, e = a.store.loadOptirout()
			if e != nil {
				jsonReply(w, 500, Object{"error": e.Error()})
				return true
			}
		}
		groups = cfg["groups"]
	} else {
		groups = payload["groups"]
		if groups == nil {
			groups = obj(a.store.snapshot()["routing"])["groups_ui"]
		}
	}
	blob, e := xlsxBytes(routingRows(groups, optirout))
	if e != nil {
		jsonReply(w, 500, Object{"error": e.Error()})
		return true
	}
	name := backupName(str(a.store.snapshot()["proxy_name"]), kind, "xlsx", time.Now())
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	w.Header().Set("Content-Length", fmt.Sprint(len(blob)))
	_, _ = w.Write(blob)
	return true
}
