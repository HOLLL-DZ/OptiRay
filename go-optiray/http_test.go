package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFavoriteDoesNotReorderSubscriptions(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.data["subscriptions"] = []any{Object{"id": 1, "servers": []any{Object{"sid": "first"}}}, Object{"id": 2, "servers": []any{Object{"sid": "second"}}}}
	a := &App{store: s}
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/api/toggle_favorite", strings.NewReader("sid=second"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	d := s.snapshot()
	if integer(obj(list(d["subscriptions"])[0])["id"]) != 1 || len(list(d["favorites"])) != 0 {
		t.Fatal("toggle changed ordering or did not remove favorite")
	}
}
func TestRepeatedReorderIDsDoNotDuplicateSubscriptions(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.data["subscriptions"] = []any{Object{"id": 1}, Object{"id": 2}}
	a := &App{store: s}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/reorder_subs", strings.NewReader(`{"order":[2,2]}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	subs := list(s.snapshot()["subscriptions"])
	if len(subs) != 2 || integer(obj(subs[0])["id"]) != 2 {
		t.Fatal(subs)
	}
}
func TestDisabledConnectionDoesNotReportSuccess(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/select", strings.NewReader("sid=test")))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal(w.Code)
	}
	if str(s.snapshot()["selected"]) != "" {
		t.Fatal("selected server changed")
	}
}
