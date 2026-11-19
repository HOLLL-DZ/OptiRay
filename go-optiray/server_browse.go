package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

func validServerBrowse(state Object) bool {
	filter, sort := str(state["filter"]), str(state["sort"])
	return (filter == "all" || filter == "favorites" || filter == "available") && (sort == "original" || sort == "name" || sort == "latency")
}
func (a *App) saveServerBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonReply(w, 405, Object{"ok": false, "error": "POST required"})
		return
	}
	state := Object{}
	err := json.NewDecoder(r.Body).Decode(&state)
	if err == nil && !validServerBrowse(state) {
		err = fmt.Errorf("invalid server filter or sort")
	}
	if err == nil {
		clean := Object{"filter": state["filter"], "sort": state["sort"]}
		a.connectMu.Lock()
		current := a.store.snapshot()
		if boolean(current["remember_server_browse"]) && !reflect.DeepEqual(obj(current["server_browse"]), clean) {
			err = a.store.change(func(d Object) error { d["server_browse"] = clean; return nil })
		}
		a.connectMu.Unlock()
	}
	if err != nil {
		jsonReply(w, 400, Object{"ok": false, "error": err.Error()})
		return
	}
	jsonReply(w, 200, Object{"ok": true})
}
