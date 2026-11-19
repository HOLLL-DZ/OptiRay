package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerBrowseConfigurationPersistence(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	post := func(body string) int {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("POST", "/api/server_browse", strings.NewReader(body)))
		return w.Code
	}
	value := `{"query":"Japan","filter":"favorites","sort":"latency"}`
	if post(value) != 200 || len(obj(s.snapshot()["server_browse"])) != 0 {
		t.Fatal("disabled preference persisted")
	}
	if err = s.change(func(d Object) error { d["remember_server_browse"] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if post(value) != 200 {
		t.Fatal("save failed")
	}
	reloaded, err := newStore(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if str(obj(reloaded.snapshot()["server_browse"])["filter"]) != "favorites" {
		t.Fatal("state did not survive reload")
	}
	if _, exists := obj(reloaded.snapshot()["server_browse"])["query"]; exists {
		t.Fatal("search query persisted")
	}
	if post(`{"query":"","filter":"wrong","sort":"name"}`) != 400 {
		t.Fatal("invalid filter accepted")
	}
	if str(obj(s.snapshot()["server_browse"])["filter"]) != "favorites" {
		t.Fatal("invalid request modified saved state")
	}
}
