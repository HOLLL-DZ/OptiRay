package main

import (
	"os"
	"path/filepath"
	"sync"
)

const diagnosticLogLimit = 512 << 10

var diagnosticPolicies sync.Map

type diagnosticPolicy struct {
	sync.Mutex
	enabled bool
}

func policyFor(dir string) *diagnosticPolicy {
	p, _ := diagnosticPolicies.LoadOrStore(dir, &diagnosticPolicy{})
	return p.(*diagnosticPolicy)
}
func setDiagnosticLogging(dir string, enabled bool) {
	p := policyFor(dir)
	p.Lock()
	defer p.Unlock()
	p.enabled = enabled
	if !enabled {
		for _, name := range []string{"manager.log", "manager.log.1", "xray-go.log", "xray-go.log.1", "routing/engine.log", "routing/engine.log.1", "log/manager.log", "log/manager.log.1", "log/xray-go.log", "log/xray-go.log.1", "log/engine.log", "log/engine.log.1"} {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

type diagnosticWriter struct{ dir, name string }

func newDiagnosticWriter(dir, name string) *diagnosticWriter { return &diagnosticWriter{dir, name} }
func (w *diagnosticWriter) Close() error                     { return nil }
func (w *diagnosticWriter) Write(data []byte) (int, error) {
	p := policyFor(w.dir)
	p.Lock()
	defer p.Unlock()
	n := len(data)
	if !p.enabled {
		return n, nil
	}
	path := filepath.Join(w.dir, w.name)
	if len(data) > diagnosticLogLimit {
		data = data[len(data)-diagnosticLogLimit:]
	}
	if info, e := os.Stat(path); e == nil && info.Size()+int64(len(data)) > diagnosticLogLimit {
		_ = os.Remove(path + ".1")
		if e = os.Rename(path, path+".1"); e != nil {
			return 0, e
		}
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return 0, e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return 0, e
	}
	defer f.Close()
	if _, e = f.Write(data); e != nil {
		return 0, e
	}
	return n, nil
}
