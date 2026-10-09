package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

func (a *App) supervise(ctx context.Context) {
	recoverOwned := func() {
		if !a.connectMu.TryLock() {
			return
		}
		defer a.connectMu.Unlock()
		d := a.store.snapshot()
		if a.routing != nil && a.routingWanted(d) && !boolean(a.routing.Status()["running"]) {
			if _, e := os.Stat(a.routing.bin); e == nil {
				if _, e = a.store.loadOptirout(); e == nil {
					if e = a.routing.Restart(ctx); e == nil {
						_ = os.Remove(filepath.Join(a.store.dir, "shutdown-error"))
					}
					if e != nil && boolean(d["enable_logging"]) {
						log.Printf("Owned routing startup failed: %v", e)
					}
				}
			}
		}
		if optirayEnabled(d) && a.core != nil && !a.core.Running() {
			srv, _ := findServer(d, str(d["selected"]), "")
			if srv != nil {
				if e := a.core.ApplyServer(srv, d, integer(d["proxy_port"])); e != nil && boolean(d["enable_logging"]) {
					log.Printf("Owned core recovery failed: %v", e)
				}
			}
		}
	}
	recoverOwned()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			recoverOwned()
		}
	}
}
