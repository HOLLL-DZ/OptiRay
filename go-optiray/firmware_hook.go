package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func netfilterHook(ctx context.Context, dir, kind, table string) error {
	socket := filepath.Join(dir, "routing", "engine.sock")
	if _, e := os.Stat(socket); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	body, e := json.Marshal(Object{"type": kind, "table": table})
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	req, e := http.NewRequestWithContext(ctx, "POST", "http://localhost/api/v1/system/hooks/netfilterd", bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	response, e := (&http.Client{Transport: transport, Timeout: 3 * time.Second}).Do(req)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode != 200 {
		return fmt.Errorf("routing hook HTTP %d", response.StatusCode)
	}
	return nil
}
