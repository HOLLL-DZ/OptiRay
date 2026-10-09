package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEntwareCertificatesPreserveExplicitTrustAndDiscoverBundle(t *testing.T) {
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("SSL_CERT_DIR", "")
	bundle := filepath.Join(t.TempDir(), "ca.pem")
	configureEntwareCertificates(bundle)
	if os.Getenv("SSL_CERT_FILE") != "" {
		t.Fatal("used nonexistent certificate bundle")
	}
	if e := os.WriteFile(bundle, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	configureEntwareCertificates(bundle)
	if os.Getenv("SSL_CERT_FILE") != bundle {
		t.Fatal("Entware bundle not discovered")
	}
	t.Setenv("SSL_CERT_FILE", "explicit.pem")
	configureEntwareCertificates(bundle)
	if os.Getenv("SSL_CERT_FILE") != "explicit.pem" {
		t.Fatal("explicit trust overridden")
	}
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("SSL_CERT_DIR", "custom-certs")
	configureEntwareCertificates(bundle)
	if os.Getenv("SSL_CERT_FILE") != "" {
		t.Fatal("explicit certificate directory overridden")
	}
}
