package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGeoCategoryIndexAndAttributes(t *testing.T) {
	attr := geoMessage(26, geoMessage(10, []byte("ads")))
	domain := append([]byte{8, 2}, geoMessage(18, []byte("example.com"))...)
	domain = append(domain, attr...)
	entry := append(geoMessage(10, []byte("TEST")), geoMessage(18, domain)...)
	categories, err := parseGeoCategories(geoMessage(10, entry))
	if err != nil || len(categories) != 1 || categories[0].Code != "test" || categories[0].Count != 1 || len(categories[0].Attributes) != 1 || categories[0].Attributes[0] != "ads" {
		t.Fatalf("bad index: %#v %v", categories, err)
	}
	for _, data := range [][]byte{nil, []byte("html"), {10, 99, 10, 1, 'a'}} {
		if _, err := parseGeoCategories(data); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
}
func TestGeoCategoriesMissingAndReplacedFile(t *testing.T) {
	store, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	read := func() Object {
		w := httptest.NewRecorder()
		app.geoCategories(w, httptest.NewRequest("GET", "/api/geo_categories", nil))
		out := Object{}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if boolean(obj(read()["geoip"])["exists"]) {
		t.Fatal("missing database reported installed")
	}
	root := filepath.Join(store.dir, "bin")
	os.MkdirAll(root, 0700)
	path := filepath.Join(root, "geoip.dat")
	os.WriteFile(path, geoSample("geoip.dat"), 0600)
	info := obj(read()["geoip"])
	if len(list(info["categories"])) != 1 {
		t.Fatal(info)
	}
	entry := append(geoMessage(10, []byte("OTHER")), geoMessage(18, []byte{10, 4, 10, 0, 0, 0, 16, 8})...)
	next := path + ".next"
	os.WriteFile(next, geoMessage(10, entry), 0600)
	os.Remove(path)
	os.Rename(next, path)
	info = obj(read()["geoip"])
	if str(obj(list(info["categories"])[0])["code"]) != "other" {
		t.Fatal("stale cached categories", info)
	}
	os.Remove(path)
	if boolean(obj(read()["geoip"])["exists"]) {
		t.Fatal("removed database returned stale catalog")
	}
}
