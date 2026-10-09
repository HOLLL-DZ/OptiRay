package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

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
