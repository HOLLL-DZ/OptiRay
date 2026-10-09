package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type proxyInterface struct {
	Name, Host, Protocol, Block string
	Port                        int
}
type ndmcRunner func(context.Context, string) (string, error)
type Keenetic struct {
	mu  sync.Mutex
	dir string
	run ndmcRunner
}

func parseProxies(text string) map[string]proxyInterface {
	out := map[string]proxyInterface{}
	id := ""
	lines := []string{}
	flush := func() {
		if id == "" {
			return
		}
		p := proxyInterface{Block: strings.Join(lines, "\n") + "\n"}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "description ") {
				p.Name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description ")), `"`)
			}
			fields := strings.Fields(line)
			if len(fields) == 4 && fields[0] == "proxy" && fields[1] == "upstream" {
				p.Host = fields[2]
				p.Port, _ = strconv.Atoi(fields[3])
			}
			if len(fields) == 3 && fields[0] == "proxy" && fields[1] == "protocol" {
				p.Protocol = fields[2]
			}
		}
		out[id] = p
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "interface ") {
			flush()
			id = ""
			lines = nil
			fields := strings.Fields(line)
			if len(fields) > 1 && regexp.MustCompile(`^Proxy[0-9]+$`).MatchString(fields[1]) {
				id = fields[1]
			}
			continue
		}
		if strings.HasPrefix(line, "!") || len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			flush()
			id = ""
			lines = nil
			continue
		}
		if id != "" {
			lines = append(lines, line)
		}
	}
	flush()
	return out
}
func commandHasError(text string) bool {
	pattern := regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9_:]*\s+(?:error\[|\(error\))|unknown command\b|invalid command\b)`)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "!") {
			continue
		}
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}
func executableNDMC(bin string) ndmcRunner {
	return func(ctx context.Context, command string) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		output, e := exec.CommandContext(ctx, bin, "-c", command).Output()
		if e != nil || commandHasError(string(output)) {
			return "", fmt.Errorf("Keenetic did not apply command")
		}
		return string(output), nil
	}
}
func validProxyParams(name, host string, port int) bool {
	return strings.TrimSpace(name) != "" && !strings.ContainsAny(name, "\"\\\r\n\x00;") && host != "" && !strings.ContainsAny(host, " \t\r\n\"\\\x00;") && port > 0 && port <= 65535
}

// Apply keeps the first original block in a private journal. The returned undo
// restores this particular operation; it does not reset the router globally.
func (k *Keenetic) Apply(ctx context.Context, name, host string, port int) (func() error, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !validProxyParams(name, host, port) {
		return nil, fmt.Errorf("invalid Keenetic proxy parameters")
	}
	text, e := k.run(ctx, "show running-config")
	if e != nil {
		return nil, e
	}
	interfaces := parseProxies(text)
	ids := []string{}
	for id, p := range interfaces {
		if p.Name == name {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 1 {
		return nil, fmt.Errorf("multiple proxies have the requested name")
	}
	managedPath := filepath.Join(k.dir, "proxy-managed.json")
	managed, e := readObject(managedPath)
	if os.IsNotExist(e) {
		managed = Object{}
	} else if e != nil {
		return nil, e
	}
	target := ""
	if len(ids) == 1 {
		target = ids[0]
	} else {
		for id, value := range managed {
			old := obj(value)
			p, ok := interfaces[id]
			if ok && p.Name == str(old["proxy_name"]) && p.Host == str(old["proxy_host"]) && p.Port == integer(old["proxy_port"]) && p.Protocol == "socks5" {
				if target != "" {
					return nil, fmt.Errorf("multiple managed proxies")
				}
				target = id
			}
		}
		for i := 0; target == "" && i < 16; i++ {
			id := fmt.Sprintf("Proxy%d", i)
			if _, exists := interfaces[id]; !exists {
				target = id
				break
			}
		}
	}
	if target == "" {
		return nil, fmt.Errorf("no free Proxy0-15 interface")
	}
	original, exists := interfaces[target]
	// Do not repurpose a namesake with different upstream settings in test mode.
	baseline := obj(managed[target])
	owned := original.Name == str(baseline["proxy_name"]) && original.Host == str(baseline["proxy_host"]) && original.Port == integer(baseline["proxy_port"]) && original.Protocol == "socks5"
	if exists && !owned && (original.Host != host || original.Port != port || original.Protocol != "socks5") {
		return nil, fmt.Errorf("existing namesake proxy has different settings; refusing to overwrite it")
	}
	journalPath := filepath.Join(k.dir, "proxy-original.json")
	journal, e := readObject(journalPath)
	if os.IsNotExist(e) {
		journal = Object{}
	} else if e != nil {
		return nil, e
	}
	if _, known := journal[target]; !known {
		if exists {
			journal[target] = original.Block
		} else {
			journal[target] = nil
		}
		if e = writeObject(journalPath, journal); e != nil {
			return nil, e
		}
	}
	rollback := func() error {
		undoCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if !exists {
			if _, e := k.run(undoCtx, "no interface "+target); e != nil {
				return e
			}
		} else {
			for _, line := range strings.Split(original.Block, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if _, e := k.run(undoCtx, "interface "+target+" "+line); e != nil {
					return e
				}
			}
		}
		_, e := k.run(undoCtx, "system configuration save")
		return e
	}
	commands := []string{}
	if !exists {
		commands = append(commands, "interface "+target)
	}
	if !exists {
		commands = append(commands, "interface "+target+" security-level public", "interface "+target+" ip global 1", "interface "+target+" up")
	}
	commands = append(commands, "interface "+target+" proxy protocol socks5", fmt.Sprintf("interface %s proxy upstream %s %d", target, host, port), fmt.Sprintf("interface %s description \"%s\"", target, name))
	if exists && original.Name == name && original.Host == host && original.Port == port && original.Protocol == "socks5" {
		commands = nil
	}
	for _, command := range commands {
		if _, e = k.run(ctx, command); e != nil {
			if undoError := rollback(); undoError != nil {
				return nil, fmt.Errorf("proxy apply failed and rollback failed")
			}
			return nil, e
		}
	}
	applied, e := k.run(ctx, "show running-config")
	p, found := parseProxies(applied)[target]
	if e != nil || !found || p.Name != name || p.Host != host || p.Port != port || p.Protocol != "socks5" {
		if undoError := rollback(); undoError != nil {
			return nil, fmt.Errorf("proxy verification and rollback failed")
		}
		return nil, fmt.Errorf("proxy verification failed")
	}
	if _, e = k.run(ctx, "system configuration save"); e != nil {
		_ = rollback()
		return nil, e
	}
	managed[target] = Object{"proxy_name": name, "proxy_host": host, "proxy_port": port}
	if e = writeObject(managedPath, managed); e != nil {
		_ = rollback()
		return nil, e
	}
	return func() error {
		k.mu.Lock()
		defer k.mu.Unlock()
		if e := rollback(); e != nil {
			return e
		}
		if len(baseline) == 0 {
			delete(managed, target)
		} else {
			managed[target] = baseline
		}
		return writeObject(managedPath, managed)
	}, nil
}
