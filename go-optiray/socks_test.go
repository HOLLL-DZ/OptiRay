package main

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestSocksUsesRemoteDNS(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	observed := make(chan string, 1)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			observed <- "accept failed"
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		greeting := make([]byte, 3)
		if _, e = io.ReadFull(conn, greeting); e != nil {
			observed <- "greeting failed"
			return
		}
		_, _ = conn.Write([]byte{5, 0})
		header := make([]byte, 5)
		if _, e = io.ReadFull(conn, header); e != nil || header[3] != 3 {
			observed <- "not a domain request"
			return
		}
		body := make([]byte, int(header[4])+2)
		_, e = io.ReadFull(conn, body)
		if e != nil || binary.BigEndian.Uint16(body[len(body)-2:]) != 443 {
			observed <- "invalid port"
			return
		}
		observed <- string(body[:len(body)-2])
		_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, e := socksDial(ctx, listener.Addr().String(), "unresolved.invalid:443")
	if e != nil {
		t.Fatal(e)
	}
	conn.Close()
	if host := <-observed; host != "unresolved.invalid" {
		t.Fatal(host)
	}
}
