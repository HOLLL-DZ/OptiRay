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

func TestFetchStandaloneConfigurationIgnoresSubscriptionHWID(t *testing.T) {
	servers, name, err := fetchSubscription(context.Background(), "vless://test-uuid@example.com:443?security=reality&type=tcp&pbk=public&sni=example.com#Standalone", "", "device-id", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || name != "Standalone" {
		t.Fatalf("unexpected standalone result: %d %q", len(servers), name)
	}
	stream := obj(obj(obj(servers[0])["outbound"])["streamSettings"])
	if str(obj(stream["realitySettings"])["publicKey"]) != "public" {
		t.Fatal("Reality settings lost")
	}
}

func TestStandaloneConfigurationGrouping(t *testing.T) {
    uri := "vless://test-uuid@example.com:443?security=tls#Manual"
    d := Object{"subscriptions":[]any{Object{"id":1,"url":uri},Object{"id":2,"url":"https://example.com/sub"},Object{"id":3,"url":uri}}}
    if err := collectStandaloneConfigs(d); err != nil { t.Fatal(err) }
    subs := list(d["subscriptions"])
    if len(subs)!=2 || !boolean(obj(subs[0])["standalone_group"]) || len(list(obj(subs[0])["servers"]))!=1 { t.Fatal("grouping or deduplication failed") }
    if err := collectStandaloneConfigs(d); err != nil { t.Fatal(err) }
    if len(list(d["subscriptions"]))!=2 { t.Fatal("grouping is not idempotent") }
}
func TestStandaloneConfigurationHasPingIdentity(t *testing.T) {
 d := Object{"subscriptions":[]any{Object{"id":1,"url":"vless://first@example.com:443#First"},Object{"id":2,"url":"vless://second@example.com:443#Second"}}}
 if err := collectStandaloneConfigs(d); err != nil { t.Fatal(err) }
 servers := list(obj(list(d["subscriptions"])[0])["servers"])
 if len(servers)!=2 { t.Fatal("distinct configurations were lost") }
 for _, entry := range servers { if str(obj(entry)["sid"])=="" { t.Fatal("missing ping identity") } }
}