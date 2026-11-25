package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGeoDownloadReportsBytes(t *testing.T) {
	body := bytes.Repeat([]byte("data"), 32768)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer server.Close()
	var done int64
	err := downloadCoreProgress(context.Background(), server.URL, filepath.Join(t.TempDir(), "geo.dat"), func(n, total int64) {
		if n < done {
			t.Error("progress moved backwards")
		}
		done = n
	})
	if err != nil || done != int64(len(body)) {
		t.Fatalf("download: %v, bytes %d", err, done)
	}
}

func TestGeoTransactionRestoresFileReference(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	old := bytes.Repeat([]byte("old"), 1024*1024)
	ref := ".settings-backup-geosite.dat"
	if err := os.WriteFile(filepath.Join(dir, ref), old, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "bin", "geosite.dat")
	if err := os.WriteFile(target, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeObject(filepath.Join(dir, ".settings-transaction.json"), Object{"bin/geosite.dat": Object{"file": ref}}); err != nil {
		t.Fatal(err)
	}
	if err := recoverSettings(dir); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(restored, old) {
		t.Fatal("original DAT was not restored")
	}
}
