package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVlessRealityAndIPv6(t *testing.T) {
	s, e := parseLink("vless://test-uuid@[2001:db8::1]:443?security=reality&type=grpc&sni=example.com&pbk=public&sid=1234&serviceName=tunnel#Germany")
	if e != nil {
		t.Fatal(e)
	}
	if str(s["host"]) != "2001:db8::1" || integer(s["port"]) != 443 {
		t.Fatal(s)
	}
	stream := obj(obj(s["outbound"])["streamSettings"])
	if str(obj(stream["realitySettings"])["publicKey"]) != "public" || str(obj(stream["grpcSettings"])["serviceName"]) != "tunnel" {
		t.Fatal(stream)
	}
}
func TestSubscriptionBase64AndServerIdentity(t *testing.T) {
	uri := "vless://test-uuid@example.com:443?security=tls#Germany"
	s, e := parseLink(uri)
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := parseSubscription(base64.StdEncoding.EncodeToString([]byte(uri)))
	if e != nil {
		t.Fatal(e)
	}
	if len(a) != 1 || str(obj(a[0])["sid"]) != serverID(s) {
		t.Fatal("identity changed across formats")
	}
}
func TestFetchUsesHappAndPersistentHWID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("User-Agent") != "Happ/4.7.1 (Linux; Android 16)" || r.Header.Get("x-hwid") != "existing-id" || r.URL.Query().Get("hwid") != "existing-id" {
			t.Error("incorrect subscription identity")
		}
		w.Header().Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte("Подписка")))
		_, _ = w.Write([]byte("vless://test-uuid@example.com:443?security=tls#Germany"))
	}))
	defer srv.Close()
	servers, title, e := fetchSubscription(context.Background(), srv.URL, "happ", "existing-id", true)
	if e != nil || !called || len(servers) != 1 || title != "Подписка" {
		t.Fatalf("%v %s %v", servers, title, e)
	}
}
func TestInvalidSubscriptionIsAnError(t *testing.T) {
	for _, value := range []string{"", "<html>error</html>", "[]", "{}"} {
		if _, _, e := parseSubscription(value); e == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
