package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnosticLoggingOffAndRuntimeToggle(t *testing.T) {
	dir := t.TempDir()
	w := newDiagnosticWriter(dir, "log/engine.log")
	w.Write([]byte("off"))
	if _, e := os.Stat(filepath.Join(dir, "log/engine.log")); !os.IsNotExist(e) {
		t.Fatal("disabled log created")
	}
	setDiagnosticLogging(dir, true)
	w.Write([]byte("on"))
	if b, e := os.ReadFile(filepath.Join(dir, "log/engine.log")); e != nil || string(b) != "on" {
		t.Fatal("enabled log missing", e)
	}
	setDiagnosticLogging(dir, false)
	w.Write([]byte("off"))
	if _, e := os.Stat(filepath.Join(dir, "log/engine.log")); !os.IsNotExist(e) {
		t.Fatal("disabled log remains")
	}
}
func TestDiagnosticLogBound(t *testing.T) {
	dir := t.TempDir()
	setDiagnosticLogging(dir, true)
	w := newDiagnosticWriter(dir, "log/manager.log")
	block := bytes.Repeat([]byte("x"), diagnosticLogLimit+100)
	for i := 0; i < 3; i++ {
		if _, e := w.Write(block); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"log/manager.log", "log/manager.log.1"} {
		info, e := os.Stat(filepath.Join(dir, name))
		if e != nil || info.Size() > diagnosticLogLimit {
			t.Fatal(name, e)
		}
	}
}
func TestLoggingMigrationDefaultsOffOnce(t *testing.T) {
	dir := t.TempDir()
	writeObject(filepath.Join(dir, "subs.json"), Object{"hwid": "test", "enable_logging": true})
	s, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	if boolean(s.snapshot()["enable_logging"]) {
		t.Fatal("old debug enabled")
	}
	if e = s.change(func(d Object) error { d["enable_logging"] = true; return nil }); e != nil {
		t.Fatal(e)
	}
	s, e = newStore(dir)
	if e != nil || !boolean(s.snapshot()["enable_logging"]) {
		t.Fatal("explicit enable lost", e)
	}
}
