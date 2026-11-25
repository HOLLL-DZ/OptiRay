package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
)

func uiPreferences(d Object) Object {
	colors := Object{}
	for _, v := range list(d["subscriptions"]) {
		sub := obj(v)
		if id, color := str(sub["id"]), str(sub["color"]); id != "" && color != "" {
			colors[id] = color
		}
	}
	browse := Object{}
	if state := obj(d["server_browse"]); validServerBrowse(state) {
		browse = Object{"filter": state["filter"], "sort": state["sort"]}
	}
	return Object{"subscription_colors": colors, "server_browse": browse}
}

func loadUIPreferences(dir string, d Object) {
	delete(obj(d["server_browse"]), "query")
	path := filepath.Join(dir, "ui.json")
	ui, err := readObject(path)
	if os.IsNotExist(err) {
		// Keep the legacy settings available for older managers and failed migrations.
		err = writeObject(path, uiPreferences(d))
	}
	if err != nil {
		log.Printf("Load interface preferences: %v", err)
		return
	}
	if ui == nil {
		return
	}
	colors := obj(ui["subscription_colors"])
	for _, v := range list(d["subscriptions"]) {
		sub := obj(v)
		if color, ok := colors[str(sub["id"])].(string); ok {
			sub["color"] = color
		}
	}
	if state, ok := ui["server_browse"]; ok {
		browse := obj(state)
		delete(browse, "query")
		if len(browse) == 0 || validServerBrowse(browse) {
			d["server_browse"] = browse
		}
	}
}

func (s *Store) setSubscriptionColor(id, color string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range list(s.data["subscriptions"]) {
		sub := obj(v)
		if str(sub["id"]) != id {
			continue
		}
		if str(sub["color"]) == color {
			return nil
		}
		ui := uiPreferences(s.data)
		obj(ui["subscription_colors"])[id] = color
		if err := writeObject(filepath.Join(s.dir, "ui.json"), ui); err != nil {
			return err
		}
		sub["color"] = color
		return nil
	}
	return fmt.Errorf("subscription not found")
}

func (s *Store) setServerBrowse(state Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !boolean(s.data["remember_server_browse"]) || reflect.DeepEqual(obj(s.data["server_browse"]), state) {
		return nil
	}
	ui := uiPreferences(s.data)
	ui["server_browse"] = state
	if err := writeObject(filepath.Join(s.dir, "ui.json"), ui); err != nil {
		return err
	}
	s.data["server_browse"] = state
	return nil
}
