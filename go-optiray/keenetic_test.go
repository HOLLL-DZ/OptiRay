package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

type fakeKeenetic struct {
	proxies  map[string]string
	commands []string
	fail     string
}

func (f *fakeKeenetic) run(_ context.Context, command string) (string, error) {
	f.commands = append(f.commands, command)
	if f.fail != "" && strings.Contains(command, f.fail) {
		return "", fmt.Errorf("injected command failure")
	}
	if command == "show running-config" {
		var out strings.Builder
		out.WriteString("!Network::Interface ERROR[0]: stale diagnostic comment\n")
		for id, block := range f.proxies {
			fmt.Fprintf(&out, "interface %s\n%s!\n", id, block)
		}
		return out.String(), nil
	}
	fields := strings.Fields(command)
	if len(fields) >= 3 && fields[0] == "no" && fields[1] == "interface" {
		delete(f.proxies, fields[2])
		return "", nil
	}
	if len(fields) >= 2 && fields[0] == "interface" {
		id := fields[1]
		if len(fields) == 2 {
			f.proxies[id] = ""
			return "", nil
		}
		line := strings.Join(fields[2:], " ")
		prefix := fields[2]
		if prefix == "proxy" && len(fields) > 3 {
			prefix += " " + fields[3]
		}
		lines := []string{}
		for _, old := range strings.Split(f.proxies[id], "\n") {
			if old != "" && !strings.HasPrefix(strings.TrimSpace(old), prefix+" ") {
				lines = append(lines, old)
			}
		}
		lines = append(lines, " "+line)
		f.proxies[id] = strings.Join(lines, "\n") + "\n"
	}
	return "", nil
}
func TestKeeneticCreatesVerifiesAndJournalsProxy(t *testing.T) {
	fake := &fakeKeenetic{proxies: map[string]string{"Proxy0": " description Other\n proxy protocol socks5\n proxy upstream 10.0.0.1 1080\n"}}
	k := &Keenetic{dir: t.TempDir(), run: fake.run}
	undo, e := k.Apply(context.Background(), "OptiRay_Admin", "192.168.1.1", 3081)
	if e != nil {
		t.Fatal(e)
	}
	p := parseProxies("interface Proxy1\n" + fake.proxies["Proxy1"])["Proxy1"]
	if p.Name != "OptiRay_Admin" || p.Port != 3081 {
		t.Fatal(p)
	}
	journal, e := readObject(filepath.Join(k.dir, "proxy-original.json"))
	if e != nil {
		t.Fatal(e)
	}
	if v, ok := journal["Proxy1"]; !ok || v != nil {
		t.Fatal("created interface journal incorrect")
	}
	if _, e = k.Apply(context.Background(), "OptiRay_Admin", "192.168.1.1", 3081); e != nil {
		t.Fatal(e)
	}
	journal, _ = readObject(filepath.Join(k.dir, "proxy-original.json"))
	if journal["Proxy1"] != nil {
		t.Fatal("first original overwritten")
	}
	if e = undo(); e != nil {
		t.Fatal(e)
	}
	if _, ok := fake.proxies["Proxy1"]; ok {
		t.Fatal("undo left created proxy")
	}
	if !strings.Contains(fake.proxies["Proxy0"], "10.0.0.1") {
		t.Fatal("foreign proxy changed")
	}
}
func TestKeeneticRollsBackFailedCreation(t *testing.T) {
	fake := &fakeKeenetic{proxies: map[string]string{}, fail: "proxy upstream"}
	k := &Keenetic{dir: t.TempDir(), run: fake.run}
	if _, e := k.Apply(context.Background(), "OptiRay", "192.168.1.1", 3081); e == nil {
		t.Fatal("failure ignored")
	}
	if len(fake.proxies) != 0 {
		t.Fatal("partial proxy remains")
	}
}
func TestKeeneticRejectsNamesakeAndInjection(t *testing.T) {
	fake := &fakeKeenetic{proxies: map[string]string{"Proxy0": " description OptiRay\n proxy protocol socks5\n proxy upstream 10.0.0.1 1080\n"}}
	k := &Keenetic{dir: t.TempDir(), run: fake.run}
	if _, e := k.Apply(context.Background(), "OptiRay", "192.168.1.1", 3081); e == nil {
		t.Fatal("foreign namesake overwritten")
	}
	for _, command := range fake.commands {
		if command != "show running-config" {
			t.Fatal("mutated foreign proxy")
		}
	}
	if _, e := k.Apply(context.Background(), "name; no interface Proxy0", "192.168.1.1", 3081); e == nil {
		t.Fatal("unsafe name accepted")
	}
}
func TestKeeneticIgnoresDiagnosticCommentsButDetectsErrors(t *testing.T) {
	if commandHasError("!Network::Interface ERROR[1]: diagnostic\ninterface Proxy0\n") {
		t.Fatal("comment treated as command error")
	}
	if !commandHasError("Network::Interface ERROR[1]: failed") {
		t.Fatal("actual command error ignored")
	}
}

func TestManagedProxyUpdatePreservesInterfacePolicyAndState(t *testing.T) {
	fake := &fakeKeenetic{proxies: map[string]string{"Proxy0": " description OptiRay\n security-level private\n ip global 7\n down\n proxy protocol socks5\n proxy upstream 192.168.1.1 3080\n"}}
	k := &Keenetic{dir: t.TempDir(), run: fake.run}
	if _, e := k.Apply(context.Background(), "OptiRay", "192.168.1.1", 3080); e != nil {
		t.Fatal(e)
	}
	fake.commands = nil
	undo, e := k.Apply(context.Background(), "Renamed", "192.168.1.1", 3081)
	if e != nil {
		t.Fatal(e)
	}
	for _, command := range fake.commands {
		if strings.Contains(command, "security-level") || strings.Contains(command, " ip global ") || strings.HasSuffix(command, " up") {
			t.Fatalf("modified existing interface policy: %s", command)
		}
	}
	block := fake.proxies["Proxy0"]
	for _, property := range []string{"security-level private", "ip global 7", "down"} {
		if !strings.Contains(block, property) {
			t.Fatalf("lost %s", property)
		}
	}
	if e = undo(); e != nil {
		t.Fatal(e)
	}
	p := parseProxies("interface Proxy0\n" + fake.proxies["Proxy0"])["Proxy0"]
	if p.Name != "OptiRay" || p.Port != 3080 {
		t.Fatal("undo did not restore proxy")
	}
}
