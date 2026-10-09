package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func socksDial(ctx context.Context, proxy, address string) (net.Conn, error) {
	conn, e := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", proxy)
	if e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		if !ok {
			conn.Close()
		}
	}()
	deadline := time.Now().Add(4 * time.Second)
	if d, exists := ctx.Deadline(); exists && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	if _, e = conn.Write([]byte{5, 1, 0}); e != nil {
		return nil, e
	}
	var reply [2]byte
	if _, e = io.ReadFull(conn, reply[:]); e != nil {
		return nil, e
	}
	if reply != [2]byte{5, 0} {
		return nil, fmt.Errorf("SOCKS authentication refused")
	}
	host, p, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	port, e := strconv.Atoi(p)
	if e != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid destination port")
	}
	request := []byte{5, 1, 0}
	ip := net.ParseIP(host)
	if v4 := ip.To4(); v4 != nil {
		request = append(request, 1)
		request = append(request, v4...)
	} else if ip != nil {
		request = append(request, 4)
		request = append(request, ip.To16()...)
	} else {
		if len(host) > 255 {
			return nil, fmt.Errorf("destination name too long")
		}
		request = append(request, 3, byte(len(host)))
		request = append(request, host...)
	}
	request = binary.BigEndian.AppendUint16(request, uint16(port))
	if _, e = conn.Write(request); e != nil {
		return nil, e
	}
	var header [4]byte
	if _, e = io.ReadFull(conn, header[:]); e != nil {
		return nil, e
	}
	if header[0] != 5 || header[1] != 0 {
		return nil, fmt.Errorf("SOCKS connect refused")
	}
	size := 0
	switch header[3] {
	case 1:
		size = 4
	case 4:
		size = 16
	case 3:
		var length [1]byte
		if _, e = io.ReadFull(conn, length[:]); e != nil {
			return nil, e
		}
		size = int(length[0])
	default:
		return nil, fmt.Errorf("invalid SOCKS address type")
	}
	if _, e = io.CopyN(io.Discard, conn, int64(size+2)); e != nil {
		return nil, e
	}
	_ = conn.SetDeadline(time.Time{})
	ok = true
	return conn, nil
}
func warmupProxy(ctx context.Context, port int, target string) bool {
	u, e := url.Parse(target)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		target = "http://1.1.1.1/cdn-cgi/trace"
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return socksDial(ctx, fmt.Sprintf("127.0.0.1:%d", port), address)
	}, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 4 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 4 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}
	for attempt := 0; attempt < 3; attempt++ {
		req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
		if e != nil {
			return false
		}
		resp, e := client.Do(req)
		if e == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 65536))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return true
			}
		}
		if ctx.Err() != nil {
			return false
		}
	}
	return false
}
