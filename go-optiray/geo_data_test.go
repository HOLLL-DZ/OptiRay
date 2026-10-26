package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func geoMessage(field byte, data []byte) []byte {
	out := []byte{field}
	out = binary.AppendUvarint(out, uint64(len(data)))
	return append(out, data...)
}
func geoSample(name string) []byte {
	var record []byte
	if name == "geoip.dat" {
		record = append(geoMessage(10, []byte{10, 0, 0, 0}), 16, 8)
	} else {
		record = append([]byte{8, 2}, geoMessage(18, []byte("example.com"))...)
	}
	entry := append(geoMessage(10, []byte("TEST")), geoMessage(18, record)...)
	return geoMessage(10, entry)
}

type geoFixtureTransport struct {
	body, archive []byte
	bodies        map[string][]byte
	paths         []string
}

func (g *geoFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	g.paths = append(g.paths, r.URL.String())
	body := g.body
	if value, ok := g.bodies[filepath.Base(r.URL.Path)]; ok {
		body = value
	}
	if len(g.archive) > 0 {
		body = g.archive
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
}
func TestGeoDatabaseTypeAndURL(t *testing.T) {
	for _, name := range geoNames {
		if err := validateGeoData(geoSample(name), name); err != nil {
			t.Fatal(err)
		}
		other := "geoip.dat"
		if name == other {
			other = "geosite.dat"
		}
		if validateGeoData(geoSample(other), name) == nil {
			t.Fatal("opposite database type accepted")
		}
		for _, body := range [][]byte{nil, []byte("<html>error</html>"), {10, 99, 10, 2, 'R', 'U'}, {10, 4, 10, 2, 'R', 'U'}} {
			if validateGeoData(body, name) == nil {
				t.Fatal("invalid database accepted")
			}
		}
		if _, err := geoURL(name, "https://raw.githubusercontent.com/example/data/release/"+name); err != nil {
			t.Fatal(err)
		}
		if _, err := geoURL(name, "https://github.com/example/data/releases/latest/download/"+other); err == nil {
			t.Fatal("wrong filename accepted")
		}
	}
	for _, address := range []string{"../etc", "http://example.com/geoip.dat", "https://user:password@example.com/geoip.dat", "https://"} {
		if _, err := geoURL("geoip.dat", address); err == nil {
			t.Fatal("invalid URL accepted")
		}
	}
}
func TestGeoIndependentUpdateRejectsRenamedWrongFile(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	fixture := &geoFixtureTransport{body: geoSample("geosite.dat")}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	root := filepath.Join(s.dir, "bin")
	os.MkdirAll(root, 0700)
	for _, name := range geoNames {
		os.WriteFile(filepath.Join(root, name), []byte("original"), 0600)
	}
	address := "https://raw.githubusercontent.com/example/data/release/geoip.dat"
	if a.updateGeo(context.Background(), "github", "geoip.dat", address) == nil {
		t.Fatal("GeoSite renamed to GeoIP accepted")
	}
	for _, name := range geoNames {
		data, _ := os.ReadFile(filepath.Join(root, name))
		if string(data) != "original" {
			t.Fatal("failed update changed a database")
		}
	}
	fixture.body = geoSample("geoip.dat")
	if err = a.updateGeo(context.Background(), "github", "geoip.dat", address); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "geosite.dat"))
	if string(data) != "original" {
		t.Fatal("other database changed")
	}
	again, err := newStore(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if str(geoFileConfig(again.snapshot(), "geoip.dat")["url"]) != address {
		t.Fatal("URL not persisted")
	}
	if str(geoFileConfig(again.snapshot(), "geosite.dat")["source"]) != "standard" {
		t.Fatal("other source changed")
	}
	if a.updateGeo(context.Background(), "github", "geosite.dat", address) == nil {
		t.Fatal("duplicate/wrong URL accepted")
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".geo-update-*"))
	if len(matches) > 0 {
		t.Fatal("temporary files leaked")
	}
}
func TestGeoStandardExtractsOnlyRequestedDatabase(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, name := range []string{"geoip.dat", "geosite.dat", "xray", "../unexpected"} {
		w, _ := archive.Create(name)
		w.Write(geoSample(name))
	}
	archive.Close()
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	fixture := &geoFixtureTransport{archive: buffer.Bytes()}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	for _, name := range geoNames {
		if err = a.updateGeo(context.Background(), "standard", name, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = os.Stat(filepath.Join(s.dir, "bin", "xray")); !os.IsNotExist(err) {
		t.Fatal("core replaced")
	}
	if !strings.Contains(fixture.paths[0], "XTLS/Xray-core/releases/latest/download/Xray-linux-") {
		t.Fatal(fixture.paths)
	}
	if len(a.geoInfo()["files"].([]Object)) != 2 {
		t.Fatal("missing metadata")
	}
}
func TestGeoLegacyRepositoryMigration(t *testing.T) {
	d := Object{"geo_data_source": "github", "geo_data_repository": "example/data"}
	if str(geoFileConfig(d, "geoip.dat")["url"]) != "https://github.com/example/data/releases/latest/download/geoip.dat" {
		t.Fatal("legacy setting lost")
	}
}

func TestGeoCombinedUpdateChecksBothBeforeReplacing(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	fixture := &geoFixtureTransport{bodies: map[string][]byte{"geoip.dat": geoSample("geoip.dat"), "geosite.dat": geoSample("geoip.dat")}}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: s}
	root := filepath.Join(s.dir, "bin")
	os.MkdirAll(root, 0700)
	for _, name := range geoNames {
		os.WriteFile(filepath.Join(root, name), []byte("original"), 0600)
	}
	urls := map[string]string{"geoip.dat": "https://raw.githubusercontent.com/example/data/release/geoip.dat", "geosite.dat": "https://raw.githubusercontent.com/example/data/release/geosite.dat"}
	if a.updateGeoFiles(context.Background(), "github", urls) == nil {
		t.Fatal("wrong second database accepted")
	}
	for _, name := range geoNames {
		data, _ := os.ReadFile(filepath.Join(root, name))
		if string(data) != "original" {
			t.Fatal("partial replacement on failure")
		}
	}
	fixture.bodies["geosite.dat"] = geoSample("geosite.dat")
	if err = a.updateGeoFiles(context.Background(), "github", urls); err != nil {
		t.Fatal(err)
	}
	for _, name := range geoNames {
		data, _ := os.ReadFile(filepath.Join(root, name))
		if err = validateGeoData(data, name); err != nil {
			t.Fatal(err)
		}
		if str(geoFileConfig(s.snapshot(), name)["url"]) != urls[name] {
			t.Fatal("URL not saved", name)
		}
	}
}

func TestStandardGeoUpdatePreservesCustomAddresses(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, name := range geoNames {
		w, _ := archive.Create(name)
		w.Write(geoSample(name))
	}
	archive.Close()
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	http.DefaultClient = &http.Client{Transport: &geoFixtureTransport{archive: buffer.Bytes()}}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: s}
	urls := map[string]string{"geoip.dat": "https://example.com/geoip.dat", "geosite.dat": "https://example.com/geosite.dat"}
	if err = s.change(func(d Object) error {
		d["geo_data_files"] = Object{"geoip.dat": Object{"source": "github", "url": urls["geoip.dat"]}, "geosite.dat": Object{"source": "github", "url": urls["geosite.dat"]}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = app.updateGeoFiles(context.Background(), "standard", map[string]string{"geoip.dat": "", "geosite.dat": ""}); err != nil {
		t.Fatal(err)
	}
	again, err := newStore(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range geoNames {
		cfg := geoFileConfig(again.snapshot(), name)
		if str(cfg["source"]) != "standard" || str(cfg["url"]) != urls[name] {
			t.Fatal("custom URL erased", cfg)
		}
	}
	urls["geoip.dat"] = "https://other.example/geoip.dat"
	if err = app.updateGeoFiles(context.Background(), "standard", urls); err != nil {
		t.Fatal(err)
	}
	if str(geoFileConfig(s.snapshot(), "geoip.dat")["url"]) != urls["geoip.dat"] {
		t.Fatal("entered address was not retained")
	}
}
