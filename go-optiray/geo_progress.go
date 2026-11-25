package main

import (
	"io"
	"sync"
)

type geoDownloadState struct {
	mu          sync.Mutex
	file, phase string
	done, total int64
}

func (s *geoDownloadState) set(file string, done, total int64, phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file, s.done, s.total, s.phase = file, done, total, phase
}
func (s *geoDownloadState) snapshot() Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Object{"file": s.file, "downloaded": s.done, "total": s.total, "phase": s.phase}
}

type geoProgressWriter struct {
	target      io.Writer
	done, total int64
	report      func(int64, int64)
}

func (w *geoProgressWriter) Write(b []byte) (int, error) {
	n, e := w.target.Write(b)
	w.done += int64(n)
	w.report(w.done, w.total)
	return n, e
}
