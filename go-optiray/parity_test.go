package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHWIDStableWithoutSettingsSave(t *testing.T) {
	dir := t.TempDir()
	s, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	again, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	if s.snapshot()["hwid"] != again.snapshot()["hwid"] {
		t.Fatal("first-run HWID changed")
	}
}
func TestInterruptedSettingsRecovery(t *testing.T) {
	dir := t.TempDir()
	if e := writeObject(filepath.Join(dir, ".settings-transaction.json"), Object{"subs.json": []byte(`{"old":true}`), "auth.json": nil}); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "subs.json"), []byte(`{"new":true}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := recoverSettings(dir); e != nil {
		t.Fatal(e)
	}
	d, e := readObject(filepath.Join(dir, "subs.json"))
	if e != nil || !boolean(d["old"]) {
		t.Fatal("original not recovered", e)
	}
	if _, e = os.Stat(filepath.Join(dir, "auth.json")); !os.IsNotExist(e) {
		t.Fatal("new auth remained")
	}
}
func TestRecoveryRejectsOutsidePath(t *testing.T) {
	dir := t.TempDir()
	if e := writeObject(filepath.Join(dir, ".settings-transaction.json"), Object{"../foreign": nil}); e != nil {
		t.Fatal(e)
	}
	if recoverSettings(dir) == nil {
		t.Fatal("unsafe journal accepted")
	}
}
func TestNativeAuthChallengeAndCorruption(t *testing.T) {
	t.Setenv("OPTIRAY_USER", "")
	t.Setenv("OPTIRAY_PASS", "")
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	if e = writeObject(filepath.Join(s.dir, "auth.json"), Object{"enabled": true, "user": "roman", "pass": "secret"}); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("missing challenge")
	}
	r.SetBasicAuth("roman", "secret")
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if e = os.WriteFile(filepath.Join(s.dir, "auth.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("corrupt auth allowed access")
	}
}
func TestXLSXUnicodeRoundTrip(t *testing.T) {
	rows := [][]string{{"Group", "Rule"}, {"YouTube & <test>", "youtube.com"}, {"РљРёСЂРёР»Р»РёС†Р°", "*.example"}}
	b, e := xlsxBytes(rows)
	if e != nil {
		t.Fatal(e)
	}
	got, e := parseXLSX(b)
	if e != nil || !reflect.DeepEqual(rows, got) {
		t.Fatal(got, e)
	}
}
func TestXLSXSharedAndSparseCells(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("xl/sharedStrings.xml")
	_, _ = f.Write([]byte(`<sst><si><t>Group</t></si><si><r><t>you</t></r><r><t>tube</t></r></si></sst>`))
	f, _ = z.Create("xl/worksheets/sheet1.xml")
	_, _ = f.Write([]byte(`<worksheet><sheetData><row><c r="A1" t="s"><v>0</v></c><c r="C1" t="s"><v>1</v></c></row></sheetData></worksheet>`))
	_ = z.Close()
	got, e := parseXLSX(b.Bytes())
	if e != nil || len(got) != 1 || len(got[0]) != 3 || got[0][1] != "" || got[0][2] != "youtube" {
		t.Fatal(got, e)
	}
}
func TestHysteriaURIAuthAndNativeConfig(t *testing.T) {
	s, e := parseLink("hy2://user:secret@example.org:443?sni=tls.example&obfs=salamander&obfs-password=abc#Server")
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := hysteriaConfig(s, 3081, "127.0.0.1")
	if e != nil || str(cfg["auth"]) != "user:secret" || str(obj(obj(cfg["obfs"])["salamander"])["password"]) != "abc" {
		t.Fatal(cfg, e)
	}
	other := clone(s)
	other["auth"] = "different"
	if serverID(s) == serverID(other) {
		t.Fatal("auth identities collide")
	}
}
func TestJSONCompositeSubscription(t *testing.T) {
	raw := `{"remarks":"Pool","outbounds":[{"protocol":"vless","tag":"node","settings":{"vnext":[{"address":"example.com","port":443,"users":[{"id":"uuid"}]}]}},{"protocol":"freedom","tag":"direct"}],"routing":{"balancers":[{"tag":"pool","selector":["node"]}],"rules":[{"type":"field","inboundTag":["old"],"domain":["example.org"],"outboundTag":"direct"}]},"dns":{"servers":["1.1.1.1"]}}`
	ss, _, e := parseSubscription(raw)
	if e != nil || len(ss) != 1 {
		t.Fatal(ss, e)
	}
	s := obj(ss[0])
	if !boolean(s["composite"]) || integer(s["candidates_count"]) != 1 {
		t.Fatal(s)
	}
	cfg, e := xrayConfig(s, Object{}, 3081)
	if e != nil || cfg["dns"] == nil || len(list(obj(cfg["routing"])["balancers"])) != 1 {
		t.Fatal(cfg, e)
	}
}
func TestDuplicateIdentityStableAcrossReordering(t *testing.T) {
	one := Object{"sid": "same", "outbound": Object{"tag": "one", "settings": Object{"test": "a"}}}
	two := Object{"sid": "same", "outbound": Object{"tag": "two", "settings": Object{"test": "b"}}}
	ss := []any{two, one}
	uniqueServerIDs(ss)

	if str(obj(ss[0])["sid"]) != "same-1" || str(one["sid"]) != "same" {
		t.Fatal("IDs depended on source order")
	}
}
func TestManagedProxyRenameAndUndo(t *testing.T) {
	f := &fakeKeenetic{proxies: map[string]string{}}
	k := &Keenetic{dir: t.TempDir(), run: f.run}
	if _, e := k.Apply(context.Background(), "Old", "192.168.1.1", 3081); e != nil {
		t.Fatal(e)
	}
	undo, e := k.Apply(context.Background(), "New", "192.168.1.1", 3082)
	if e != nil {
		t.Fatal(e)
	}
	if len(f.proxies) != 1 {
		t.Fatal("rename created another interface")
	}
	if e = undo(); e != nil {
		t.Fatal(e)
	}
	p := parseProxies("interface Proxy0\n" + f.proxies["Proxy0"])["Proxy0"]
	if p.Name != "Old" || p.Port != 3081 {
		t.Fatal(p)
	}
}
func TestRestoreOptiroutAndResetNative(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	d := s.snapshot()
	cfg, e := defaultOptirout()
	if e != nil {
		t.Fatal(e)
	}
	cfg["groups"] = []any{Object{"name": "Test", "rules": []any{}}}
	d["optirout_config"] = cfg
	if e = a.restoreState(context.Background(), d); e != nil {
		t.Fatal(e)
	}
	cfg, e = s.loadOptirout()
	if e != nil || len(list(cfg["groups"])) != 1 {
		t.Fatal(cfg, e)
	}
	if e = a.resetState(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestAllContractRoutesImplemented(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	b, e := assets.ReadFile("assets/route-contract.json")
	if e != nil {
		t.Fatal(e)
	}
	var paths []string
	if e = json.Unmarshal(b, &paths); e != nil {
		t.Fatal(e)
	}
	gets := map[string]bool{"/api/backup": true, "/api/hysteria_info": true, "/api/optirout_backups": true, "/api/optirout_config": true, "/api/optirout_interfaces": true, "/api/optirout_license": true, "/api/optirout_status": true, "/api/ping": true, "/api/xray_routing_config": true}
	for _, path := range paths {
		if path == "/api/xray_info" {
			continue
		}
		method := "POST"
		if gets[path] {
			method = "GET"
		}
		r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code == 404 || w.Code == 501 {
			t.Errorf("%s not implemented: %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestCoreFilesRollbackPreservesOldBinary(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	if e := os.Mkdir(stage, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "geoip.dat"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(stage, "geoip.dat"), []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := replaceCoreFiles(root, stage, []string{"geoip.dat", "xray"}); e == nil {
		t.Fatal("missing staged binary accepted")
	}
	b, e := os.ReadFile(filepath.Join(root, "geoip.dat"))
	if e != nil || string(b) != "old" {
		t.Fatal("old asset was lost", e)
	}
}
func TestRemoveManagedProxyLeavesForeignAndRefusesChanged(t *testing.T) {
	f := &fakeKeenetic{proxies: map[string]string{"Proxy1": " description Foreign\n proxy protocol socks5\n proxy upstream 10.0.0.2 1080\n"}}
	k := &Keenetic{dir: t.TempDir(), run: f.run}
	if _, e := k.Apply(context.Background(), "Owned", "192.168.1.1", 3081); e != nil {
		t.Fatal(e)
	}
	if e := k.RemoveManaged(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(f.proxies) != 1 || f.proxies["Proxy1"] == "" {
		t.Fatal("foreign proxy changed")
	}
	if _, e := k.Apply(context.Background(), "Owned", "192.168.1.1", 3081); e != nil {
		t.Fatal(e)
	}
	f.proxies["Proxy0"] = strings.ReplaceAll(f.proxies["Proxy0"], "3081", "3082")
	if e := k.RemoveManaged(context.Background()); e == nil {
		t.Fatal("outside change removed")
	}
	if len(f.proxies) != 2 {
		t.Fatal("partial deletion after preflight failure")
	}
}
func TestInterfaceDiscoveryUsesOSNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"t2s0", "nwg1", "lo", "eth0", "br0"} {
		if e := os.Mkdir(filepath.Join(dir, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	items, e := collectInterfaces(dir, "interface Proxy0\n description Managed\n!\ninterface Wireguard1\n description Tunnel\n!\ninterface Bridge0\n description LAN\n!")
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]string{}
	for _, item := range items {
		m := obj(item)
		names[str(m["name"])] = str(m["desc"])
	}
	if names["t2s0"] != "Managed" || names["nwg1"] != "Tunnel" || names["br0"] != "LAN" || len(names) != 3 {
		t.Fatal(names)
	}
}
func TestRuntimeTransactionRollsBackProxyFailure(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	f := &fakeKeenetic{proxies: map[string]string{}, fail: "proxy upstream"}
	a := &App{store: s, keenetic: &Keenetic{dir: filepath.Join(s.dir, "install-state"), run: f.run}}
	before := s.snapshot()
	next := clone(before)
	next["proxy_name"] = "New"
	if e = a.commitState(context.Background(), before, next, nil); e == nil {
		t.Fatal("failed runtime saved")
	}
	again, e := newStore(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	if str(again.snapshot()["proxy_name"]) != str(before["proxy_name"]) || len(f.proxies) != 0 {
		t.Fatal("runtime/settings rollback incomplete")
	}
}
func TestSettingsAuthTransaction(t *testing.T) {
	t.Setenv("OPTIRAY_USER", "")
	t.Setenv("OPTIRAY_PASS", "")
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a := &App{store: s}
	form := url.Values{"auth_enabled": {"1"}, "auth_user": {"roman"}, "auth_pass": {"secret"}, "subscription_client_profile": {"incy"}}
	if e = a.saveSettings(context.Background(), form); e != nil {
		t.Fatal(e)
	}
	auth, e := s.auth()
	if e != nil || !boolean(auth["enabled"]) || str(auth["user"]) != "roman" {
		t.Fatal(auth, e)
	}
	again, e := newStore(s.dir)
	if e != nil || str(again.snapshot()["subscription_client_profile"]) != "incy" {
		t.Fatal(e)
	}
}
func TestPingGateConfiguredLimitAndCancellation(t *testing.T) {
	a := &App{}
	d := Object{"ping_real_concurrency": 1}
	release, e := a.acquirePing(context.Background(), d, "real")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = a.acquirePing(ctx, d, "real"); e == nil {
		t.Fatal("cancelled queue admitted")
	}
	release()
	if done, e := a.acquirePing(context.Background(), d, "real"); e != nil {
		t.Fatal(e)
	} else {
		done()
	}
}
