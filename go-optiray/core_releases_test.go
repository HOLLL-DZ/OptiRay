package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type githubFixtureTransport struct{ body string }

func (g githubFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(g.body)), Request: r}, nil
}
func TestGitHubReleaseResponseAllowsRealMetadataSizeAndRemainsBounded(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	http.DefaultClient = &http.Client{Transport: githubFixtureTransport{body: `[{"tag_name":"v26.9.30","body":"` + strings.Repeat("a", 3<<20) + `"}]`}}
	value, e := fetchGitHubJSON(context.Background(), "releases?per_page=30")
	if e != nil || len(list(value)) != 1 {
		t.Fatalf("valid large metadata rejected: %v", e)
	}
	http.DefaultClient = &http.Client{Transport: githubFixtureTransport{body: strings.Repeat("a", maxGitHubResponse+1)}}
	if _, e = fetchGitHubJSON(context.Background(), "releases"); e == nil || !strings.Contains(e.Error(), "exceeds") {
		t.Fatalf("oversized response accepted: %v", e)
	}
}

type hysteriaReleaseFixture struct {
	fail     bool
	requests int
}

func (f *hysteriaReleaseFixture) RoundTrip(r *http.Request) (*http.Response, error) {
	f.requests++
	status := 200
	body := `[{"tag_name":"app/v2.7.1","prerelease":false},{"tag_name":"app/v2.8.0-beta.1","prerelease":true},{"tag_name":"v1.3.5"},{"tag_name":"core/v0.0.1"},{"tag_name":"app/v2.9.0","draft":true}]`
	if r.URL.Path != "/repos/apernet/hysteria/releases" && r.URL.Path != "/repos/apernet/hysteria/releases/latest" {
		return nil, fmt.Errorf("unexpected repository: %s", r.URL)
	}
	if strings.HasSuffix(r.URL.Path, "/latest") {
		body = `{"tag_name":"app/v2.7.1","prerelease":false}`
	}
	if f.fail {
		status = 503
		body = "unavailable"
	}
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}
func TestHysteriaReleasesUseOwnRepositoryAndCache(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	fixture := &hysteriaReleaseFixture{}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result := s.hysteriaReleases(context.Background(), true)
	versions := list(result["available"])
	if len(versions) != 2 {
		t.Fatalf("unexpected versions: %#v", result)
	}
	seen := map[string]bool{}
	for _, v := range versions {
		item := obj(v)
		seen[str(item["version"])] = true
	}
	if !seen["2.7.1"] || !seen["2.8.0-beta.1"] {
		t.Fatal(versions)
	}
	if fixture.requests != 2 {
		t.Fatal("expected release list and stable release")
	}
	s.hysteriaReleases(context.Background(), false)
	if fixture.requests != 2 {
		t.Fatal("cache not used")
	}
	fixture.fail = true
	result = s.hysteriaReleases(context.Background(), true)
	if str(result["versions_source"]) != "cache" || len(list(result["available"])) != 2 {
		t.Fatal("cache fallback lost", result)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "xray_releases_cache.json")); !os.IsNotExist(err) {
		t.Fatal("Hysteria changed Xray cache")
	}
}

func TestCoreReleaseOpeningNeverPollsGitHub(t *testing.T) {
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	fixture := &hysteriaReleaseFixture{}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.xrayReleases(context.Background(), false)
	s.hysteriaReleases(context.Background(), false)
	if fixture.requests != 0 {
		t.Fatal("cold cache triggered GitHub request")
	}
	s.hysteriaReleases(context.Background(), true)
	cachePath := filepath.Join(s.dir, "hysteria_releases_cache.json")
	cache, _ := readObject(cachePath)
	cache["fetched_at"] = 1
	writeObject(cachePath, cache)
	before := fixture.requests
	s.hysteriaReleases(context.Background(), false)
	if fixture.requests != before {
		t.Fatal("stale cache triggered automatic request")
	}
	fixture.fail = true
	s.hysteriaReleases(context.Background(), true)
	before = fixture.requests
	result := s.hysteriaReleases(context.Background(), false)
	if fixture.requests != before || str(result["versions_error"]) != "" {
		t.Fatal("closed settings retained a transient error or polled GitHub")
	}
	fixture.fail = false
	result = s.hysteriaReleases(context.Background(), true)
	if str(result["versions_error"]) != "" {
		t.Fatal("successful refresh retained old error")
	}
}
