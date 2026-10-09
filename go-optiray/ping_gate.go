package main

import (
	"context"
	"sync"
)

func (a *App) acquirePing(ctx context.Context, d Object, kind string) (func(), error) {
	a.pingOnce.Do(func() { a.pingChanged = make(chan struct{}) })
	limit := integer(d["ping_real_concurrency"])
	if limit < 1 || limit > 10 {
		limit = 2
	}
	if kind == "tcp" {
		limit = integer(d["ping_tcp_concurrency"])
		if limit < 1 || limit > 20 {
			limit = 5
		}
	}
	for {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		a.pingMu.Lock()
		count := &a.realActive
		if kind == "tcp" {
			count = &a.tcpActive
		}
		if *count < limit {
			*count++
			a.pingMu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					a.pingMu.Lock()
					*count--
					close(a.pingChanged)
					a.pingChanged = make(chan struct{})
					a.pingMu.Unlock()
				})
			}, nil
		}
		changed := a.pingChanged
		a.pingMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}
