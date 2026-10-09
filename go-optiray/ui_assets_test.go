package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalUIReloadsWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.css")
	store, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{uiDir: dir, store: store}
	for _, value := range []string{"body{color:red}", "body{color:green}"} {
		if e := os.WriteFile(path, []byte(value), 0600); e != nil {
			t.Fatal(e)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", "/assets/style.css", nil))
		if w.Code != 200 || w.Body.String() != value || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Body.String())
		}
	}
}
func TestEmbeddedUIHasSeparateStylesAndScript(t *testing.T) {
	a := &App{}
	b, e := a.uiAsset("page.html")
	if e != nil {
		t.Fatal(e)
	}
	for _, expected := range []string{`href="/assets/style.css"`, `src="/assets/app.js"`} {
		if !strings.Contains(string(b), expected) {
			t.Fatal("missing asset link")
		}
	}
	if _, e = a.uiAsset("../defaults.json"); e == nil {
		t.Fatal("unexpected asset accepted")
	}
}
