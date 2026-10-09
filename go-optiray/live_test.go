package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Explicit opt-in: isolated fixture, owned core processes and random loopback ports.
func TestRouterLiveCoreSmoke(t *testing.T) {
	root := os.Getenv("OPTIRAY_LIVE_FIXTURE")
	if root == "" {
		t.Skip("live fixture not enabled")
	}
	d, e := readObject(filepath.Join(root, "subs.json"))
	if e != nil {
		t.Fatal(e)
	}
	for _, sub := range list(d["subscriptions"]) {
		for _, value := range list(obj(sub)["servers"]) {
			srv := obj(value)
			t.Run(str(srv["protocol"]), func(t *testing.T) {
				ln, e := net.Listen("tcp", "127.0.0.1:0")
				if e != nil {
					t.Fatal(e)
				}
				port := ln.Addr().(*net.TCPAddr).Port
				ln.Close()
				c := &CoreManager{dir: t.TempDir(), bin: os.Getenv("OPTIRAY_LIVE_XRAY"), hysteriaBin: os.Getenv("OPTIRAY_LIVE_HYSTERIA")}
				defer c.Stop()
				if e = c.ApplyServer(srv, Object{"routing_engine": "xray", "routing": Object{}}, port); e != nil {
					if cfg, err := xrayConfig(srv, Object{}, port); err == nil {
						file := filepath.Join(c.dir, "diagnostic.json")
						_ = writeObject(file, cfg)
						output, _ := exec.Command(c.bin, "run", "-test", "-c", file).CombinedOutput()
						_ = os.WriteFile(filepath.Join(root, "preflight-error.txt"), output, 0600)
					}
					t.Fatal(e)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				latency := httpPing(ctx, port, "http://cp.cloudflare.com/generate_204", 10*time.Second)
				if latency < 0 {
					t.Fatal("HTTP through isolated core failed")
				}
				t.Logf("HTTP latency %d ms", latency)
			})
		}
	}
}
