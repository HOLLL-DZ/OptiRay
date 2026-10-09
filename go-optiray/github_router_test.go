package main

import (
	"context"
	"os"
	"testing"
)

func TestRouterGitHubTLS(t *testing.T) {
	if os.Getenv("OPTIRAY_CHECK_GITHUB") == "" {
		t.Skip("router opt-in")
	}
	configureEntwareCertificates("/opt/etc/ssl/certs/ca-certificates.crt")
	for _, endpoint := range []string{"releases?per_page=30", "releases/latest"} {
		value, e := fetchGitHubJSON(context.Background(), endpoint)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("%s OK list=%d", endpoint, len(list(value)))
	}
}
