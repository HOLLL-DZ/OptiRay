package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreVersionOutput(t *testing.T) {
	for _, sample := range []string{
		"Version:        v2.14.0\nBuildType:      release\nPlatform:       linux\nArchitecture:   mipsle\n",
		"Hysteria\nVersion: v2.13.0\n",
		"Version: v1.3.5\n",
		"Xray 26.11.9 (Xray, Penetrates Everything.)\n",
	} {
		if !coreVersionLine.MatchString(sample) {
			t.Errorf("rejected version output %q", sample)
		}
	}
	for _, sample := range []string{"", "hysteria", "error: missing config 1.2.3", "Version: invalid"} {
		if coreVersionLine.MatchString(sample) {
			t.Errorf("accepted invalid output %q", sample)
		}
	}
}

func TestInstallationCheckRejectsCorruptFilesWithoutChangingThem(t *testing.T) {
	for _, name := range []string{"subs.json", "routing/optirout.json", "core_release_schedule.json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(name))
			os.MkdirAll(filepath.Dir(path), 0700)
			data := []byte{0xd5, 0x90, 0x80, 0, 0}
			os.WriteFile(path, data, 0600)
			err := checkInstallationFiles(context.Background(), dir)
			if err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("expected named corrupt file: %v", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(data) {
				t.Fatal("preflight modified corrupt file")
			}
		})
	}
}

func TestInstallationCheckDoesNotCreateDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := checkInstallationFiles(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("preflight wrote files")
	}
}

func TestInstallationCheckRejectsUnusableCore(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bin"), 0700)
	path := filepath.Join(dir, "bin", "xray")
	os.WriteFile(path, []byte("broken executable"), 0700)
	if err := checkInstallationFiles(context.Background(), dir); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing core failure: %v", err)
	}
}
