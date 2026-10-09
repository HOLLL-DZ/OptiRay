package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

func tcpPing(ctx context.Context, host string, port int, timeout time.Duration) int {
	if host == "" || port < 1 || port > 65535 {
		return -1
	}
	start := time.Now()
	conn, e := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if e != nil {
		return -1
	}
	conn.Close()
	return int(time.Since(start).Milliseconds())
}
func httpPing(ctx context.Context, port int, target string, timeout time.Duration) int {
	u, e := url.Parse(target)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return -1
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return socksDial(ctx, fmt.Sprintf("127.0.0.1:%d", port), address)
	}, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
	if e != nil {
		return -1
	}
	start := time.Now()
	resp, e := client.Do(req)
	if e != nil {
		return -1
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 && resp.StatusCode != 301 && resp.StatusCode != 302 {
		return -1
	}
	if _, e = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20)); e != nil {
		return -1
	}
	return int(time.Since(start).Milliseconds())
}
func realPing(ctx context.Context, bin string, server Object, target string, timeout, delay time.Duration) int {
	return realPingCore(ctx, &CoreManager{bin: bin}, server, target, timeout, delay)
}
func realPingCore(ctx context.Context, binaries *CoreManager, server Object, target string, timeout, delay time.Duration) int {
	return realPingCoreSamples(ctx, binaries, server, target, timeout, delay, 1)[0]
}
func realPingCoreSamples(ctx context.Context, binaries *CoreManager, server Object, target string, timeout, delay time.Duration, count int) []int {
	samples := make([]int, count)
	for i := range samples {
		samples[i] = -1
	}
	if ctx.Err() != nil || !supportedServer(server) {
		return samples
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return samples
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	dir, e := os.MkdirTemp("", "optiray-ping-")
	if e != nil {
		return samples
	}
	defer os.RemoveAll(dir)
	core := &CoreManager{bin: binaries.bin, hysteriaBin: binaries.hysteriaBin, hysteria1Bin: binaries.hysteria1Bin, dir: dir}
	defer core.Stop()
	kind := "xray"
	var cfg Object
	if server["outbound"] == nil && !boolean(server["composite"]) {
		kind = str(server["protocol"])
		cfg, e = hysteriaConfig(server, port, "127.0.0.1")
	} else {
		cfg, e = xrayConfig(server, Object{"routing_engine": "xray", "routing": Object{}}, port)
		if e == nil {
			inbound := obj(list(cfg["inbounds"])[0])
			inbound["listen"] = "127.0.0.1"
			obj(inbound["settings"])["udp"] = false
		}
	}
	if e != nil {
		return samples
	}
	if e = core.ApplyKind(cfg, port, kind); e != nil {
		return samples
	}
	if ctx.Err() != nil {
		return samples
	}
	_ = httpPing(ctx, port, target, timeout)
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return samples
		case <-timer.C:
		}
	}
	for i := range samples {
		if ctx.Err() != nil {
			break
		}
		samples[i] = httpPing(ctx, port, target, timeout)
		if i+1 < len(samples) && delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return samples
			case <-timer.C:
			}
		}
	}
	return samples
}
func (a *App) ping(ctx context.Context, d, server Object, kind string, timeout time.Duration) int {
	release, e := a.acquirePing(ctx, d, kind)
	if e != nil {
		return -1
	}
	defer release()
	if kind == "tcp" {
		proto := str(server["protocol"])
		if proto == "hysteria" || proto == "hysteria2" {
			return -2
		}
		if timeout > 2*time.Second {
			timeout = 2 * time.Second
		}
		return tcpPing(ctx, str(server["host"]), integer(server["port"]), timeout)
	}
	if boolean(server["composite"]) && str(server["sid"]) == str(d["selected"]) && a.core != nil && a.core.Running() {
		return httpPing(ctx, integer(d["proxy_port"]), str(d["test_url"]), timeout)
	}
	if a.core == nil {
		return -1
	}
	delay := integer(d["ping_sample_delay_ms"])
	if delay < 0 {
		delay = 0
	}
	if delay > 5000 {
		delay = 5000
	}
	return realPingCore(ctx, a.core, server, str(d["test_url"]), timeout, time.Duration(delay)*time.Millisecond)
}

func (a *App) realSamples(ctx context.Context, d, server Object, timeout time.Duration) []int {
	release, e := a.acquirePing(ctx, d, "real")
	if e != nil || a.core == nil {
		if release != nil {
			release()
		}
		return []int{-1, -1, -1}
	}
	defer release()
	delay := integer(d["ping_sample_delay_ms"])
	if delay < 0 || delay > 5000 {
		delay = 300
	}
	return realPingCoreSamples(ctx, a.core, server, str(d["test_url"]), timeout, time.Duration(delay)*time.Millisecond, 3)
}
