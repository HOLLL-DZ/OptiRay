package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
)

var wifiSegmentName = regexp.MustCompile(`^br[0-9]+$`)

func (a *App) saveSettings(ctx context.Context, form url.Values) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	before := a.store.snapshot()
	next := clone(before)
	if e := applySettings(next, form); e != nil {
		return e
	}
	auth, e := a.store.auth()
	if e != nil {
		return e
	}
	auth, e = authFromForm(auth, form)
	if e != nil {
		return e
	}
	if form.Has("wifi_segments_present") {
		config, err := a.store.loadOptirout()
		if err != nil {
			return err
		}
		available := map[string]bool{}
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		for _, iface := range interfaces {
			if wifiSegmentName.MatchString(iface.Name) {
				available[iface.Name] = true
			}
		}
		selected := map[string]bool{}
		for _, name := range form["wifi_segment"] {
			if !available[name] {
				return fmt.Errorf("сегмент %s недоступен", name)
			}
			selected[name] = true
		}
		app := obj(config["app"])
		links := []any{}
		changed := false
		for _, value := range list(app["link"]) {
			name := str(value)
			if !available[name] || selected[name] {
				links = append(links, value)
			} else {
				changed = true
			}
			delete(selected, name)
		}
		for _, name := range form["wifi_segment"] {
			if selected[name] {
				links = append(links, name)
				delete(selected, name)
				changed = true
			}
		}
		if changed {
			app["link"] = links
			configBytes, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return err
			}
			authBytes, err := json.MarshalIndent(auth, "", "  ")
			if err != nil {
				return err
			}
			return a.commitState(ctx, before, next, map[string][]byte{
				"routing/optirout.json": configBytes, "auth.json": authBytes,
			})
		}
	}
	undo, e := a.applyRuntime(ctx, next)
	if e != nil {
		if rollback := a.rollbackRuntime(before, undo); rollback != nil {
			return fmt.Errorf("settings apply failed: %v; rollback failed: %v", e, rollback)
		}
		return e
	}
	authBytes, e := json.MarshalIndent(auth, "", "  ")
	if e != nil {
		return e
	}
	a.store.mu.Lock()
	currentBytes, _ := json.Marshal(a.store.data)
	beforeBytes, _ := json.Marshal(before)
	if string(currentBytes) != string(beforeBytes) {
		e = fmt.Errorf("settings changed concurrently; retry")
	} else {
		e = a.store.persistExtra(next, map[string][]byte{"auth.json": authBytes})
		if e == nil {
			a.store.data = next
		}
	}
	a.store.mu.Unlock()
	if e != nil {
		if rollback := a.rollbackRuntime(before, undo); rollback != nil {
			return fmt.Errorf("settings save failed: %v; rollback failed: %v", e, rollback)
		}
	}
	return e
}
