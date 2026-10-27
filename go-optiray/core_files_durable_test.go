package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoreStagingFailurePreservesWorkingFiles(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	os.WriteFile(filepath.Join(root, "geoip.dat"), []byte("working"), 0600)
	os.WriteFile(filepath.Join(stage, "geoip.dat"), []byte("replacement"), 0600)
	if _, err := replaceCoreFiles(root, stage, []string{"geoip.dat", "xray"}); err == nil {
		t.Fatal("missing staged binary accepted")
	}
	b, _ := os.ReadFile(filepath.Join(root, "geoip.dat"))
	if string(b) != "working" {
		t.Fatal("working file changed before all staged files were ready")
	}
}

func TestDurableCoreReplacementCanRestorePreviousFiles(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	os.WriteFile(filepath.Join(root, "xray"), []byte("previous"), 0700)
	os.WriteFile(filepath.Join(stage, "xray"), []byte("next"), 0700)
	undo, err := replaceCoreFiles(root, stage, []string{"xray"})
	if err != nil {
		t.Fatal(err)
	}
	if err = undo(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "xray"))
	if string(b) != "previous" {
		t.Fatal("rollback lost previous binary")
	}
}
