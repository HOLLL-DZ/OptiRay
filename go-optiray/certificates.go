package main

import "os"

// Go's Linux default paths do not include Entware's /opt certificate store.
// Run before the first TLS request, including requests using custom transports.
func configureEntwareCertificates(bundle string) {
	if os.Getenv("SSL_CERT_FILE") != "" || os.Getenv("SSL_CERT_DIR") != "" {
		return
	}
	if info, e := os.Stat(bundle); e == nil && info.Mode().IsRegular() && info.Size() > 0 {
		_ = os.Setenv("SSL_CERT_FILE", bundle)
	}
}
