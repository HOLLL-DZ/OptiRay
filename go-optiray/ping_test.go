package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// A test-only executable mode exercises the actual process lifecycle without
// an installed Xray, external servers or access to router configuration.
func init() {
	if os.Getenv("OPTIRAY_TEST_CORE") != "1" {
		return
	}
	if len(os.Args) < 2 || os.Args[1] != "run" {
		return
	}
	path := ""
	validate := false
	for i, arg := range os.Args {
		if arg == "-test" {
			validate = true
		}
		if arg == "-c" && i+1 < len(os.Args) {
			path = os.Args[i+1]
		}
	}
	cfg, e := readObject(path)
	if e != nil {
		os.Exit(2)
	}
	if validate {
		os.Exit(0)
	}
	inbound := obj(list(cfg["inbounds"])[0])
	listener, e := net.Listen("tcp", net.JoinHostPort(str(inbound["listen"]), str(inbound["port"])))
	if e != nil {
		os.Exit(3)
	}
	for {
		conn, e := listener.Accept()
		if e != nil {
			os.Exit(0)
		}
		go func() {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			greeting := make([]byte, 3)
			if _, e := io.ReadFull(conn, greeting); e != nil {
				return
			}
			_, _ = conn.Write([]byte{5, 0})
			header := make([]byte, 4)
			if _, e := io.ReadFull(conn, header); e != nil {
				return
			}
			size := 0
			switch header[3] {
			case 1:
				size = 4
			case 4:
				size = 16
			case 3:
				var n [1]byte
				if _, e := io.ReadFull(conn, n[:]); e != nil {
					return
				}
				size = int(n[0])
			default:
				return
			}
			if _, e := io.CopyN(io.Discard, conn, int64(size+2)); e != nil {
				return
			}
			_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
			buffer := make([]byte, 8192)
			if _, e := conn.Read(buffer); e != nil {
				return
			}
			_, _ = conn.Write([]byte("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n"))
		}()
	}
}
func TestTCPPingAndCancellation(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if tcpPing(context.Background(), "127.0.0.1", port, time.Second) < 0 {
		t.Fatal("reachable TCP listener failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if tcpPing(ctx, "127.0.0.1", port, time.Second) != -1 {
		t.Fatal("canceled ping succeeded")
	}
}
func TestPingAPIHysteriaAndMissingServer(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.data["subscriptions"] = []any{Object{"id": 1, "servers": []any{Object{"sid": "udp", "protocol": "hysteria2"}}}}
	a := &App{store: s}
	for _, test := range []struct {
		query string
		want  int
	}{{"sid=udp&type=tcp", -2}, {"sid=missing&type=real", -1}} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", "/api/ping?"+test.query, nil))
		result := Object{}
		if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
			t.Fatal(e)
		}
		if integer(result["latency"]) != test.want {
			t.Fatal(w.Body.String())
		}
	}
}
func TestIsolatedRealPingPreservesActiveCore(t *testing.T) {
	t.Setenv("OPTIRAY_TEST_CORE", "1")
	bin, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	server := Object{"sid": "active", "outbound": Object{"protocol": "vless", "tag": "proxy"}}
	cfg, e := xrayConfig(server, Object{}, port)
	if e != nil {
		t.Fatal(e)
	}
	obj(list(cfg["inbounds"])[0])["listen"] = "127.0.0.1"
	active := &CoreManager{bin: bin, dir: t.TempDir()}
	defer active.Stop()
	if e = active.Apply(cfg, port); e != nil {
		t.Fatal(e)
	}
	pid := active.cmd.Process.Pid
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if latency := realPing(ctx, bin, server, "http://unresolved.invalid/check", time.Second, 0); latency < 0 {
		t.Fatal("isolated HTTP ping failed")
	}
	if !active.Running() || active.cmd.Process.Pid != pid {
		t.Fatal("active process was replaced")
	}
	b, e := os.ReadFile(active.dir + "/xray_config.json")
	if e != nil || !strings.Contains(string(b), "vless") {
		t.Fatal("active configuration lost")
	}
}
