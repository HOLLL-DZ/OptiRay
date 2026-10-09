package main

import (
	"context"
	"io"
	"net/http"
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
