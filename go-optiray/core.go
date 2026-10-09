package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type CoreManager struct {
	mu                              sync.Mutex
	bin, dir                        string
	hysteriaBin, hysteria1Bin, kind string
	cmd                             *exec.Cmd
	done                            chan error
	last                            Object
	port                            int
	lastKind                        string
}

func (c *CoreManager) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return false
	}
	select {
	case <-c.done:
		c.cmd = nil
		return false
	default:
		return true
	}
}
func (c *CoreManager) stopLocked() {
	if c.cmd == nil {
		return
	}
	select {
	case <-c.done:
		c.cmd = nil
		return
	default:
	}
	_ = c.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-c.done:
	case <-time.After(1500 * time.Millisecond):
		_ = c.cmd.Process.Kill()
		<-c.done
	}
	c.cmd = nil
}
func (c *CoreManager) Stop() { c.mu.Lock(); defer c.mu.Unlock(); c.stopLocked() }
func (c *CoreManager) startLocked(cfg Object, port int) error {
	kind := first(c.kind, "xray")
	path := filepath.Join(c.dir, kind+"_config.json")
	if e := writeObject(path, cfg); e != nil {
		return e
	}
	logPath := filepath.Join(c.dir, "xray-go.log")
	if info, e := os.Stat(logPath); e == nil && info.Size() > 2<<20 {
		_ = os.Remove(logPath + ".1")
		_ = os.Rename(logPath, logPath+".1")
	}
	logFile, e := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	bin := c.bin
	args := []string{"run", "-c", path}
	if kind == "hysteria2" {
		bin = c.hysteriaBin
		args = []string{"client", "-c", path}
	} else if kind == "hysteria" {
		bin = c.hysteria1Bin
		args = []string{"client", "-c", path}
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = c.dir
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+filepath.Dir(c.bin))
	if e = cmd.Start(); e != nil {
		logFile.Close()
		return e
	}
	done := make(chan error, 1)
	c.cmd = cmd
	c.done = done
	go func() { done <- cmd.Wait(); logFile.Close() }()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			c.cmd = nil
			return fmt.Errorf("Xray exited before its SOCKS listener was ready; see xray-go.log")
		default:
		}
		conn, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if e == nil {
			conn.Close()
			c.last = clone(cfg)
			c.port = port
			c.lastKind = kind
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	c.stopLocked()
	return fmt.Errorf("Xray did not open SOCKS port %d", port)
}
func (c *CoreManager) Apply(cfg Object, port int) error {
	return c.ApplyKind(cfg, port, "xray")
}
func (c *CoreManager) ApplyKind(cfg Object, port int, kind string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid SOCKS port")
	}
	if c.cmd != nil {
		select {
		case <-c.done:
			c.cmd = nil
		default:
		}
	}
	if c.cmd == nil || port != c.port {
		if conn, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 150*time.Millisecond); e == nil {
			conn.Close()
			return fmt.Errorf("SOCKS port %d is already occupied by another process", port)
		}
	}
	// Validate before stopping the working core; never log subscription credentials.
	path := filepath.Join(c.dir, "xray_pending.json")
	if e := writeObject(path, cfg); e != nil {
		return e
	}
	defer os.Remove(path)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if kind == "xray" {
		check := exec.CommandContext(ctx, c.bin, "run", "-test", "-c", path)
		check.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+filepath.Dir(c.bin))
		if e := check.Run(); e != nil {
			return fmt.Errorf("Xray rejected the configuration; active core was preserved")
		}
	} else {
		bin := c.hysteriaBin
		if kind == "hysteria" {
			bin = c.hysteria1Bin
		} else if kind != "hysteria2" {
			return fmt.Errorf("unknown core")
		}
		if bin == "" {
			return fmt.Errorf("Hysteria core is not configured")
		}
		if _, e := os.Stat(bin); e != nil {
			return fmt.Errorf("Hysteria core is not installed")
		}
	}
	previous, previousPort := c.last, c.port
	previousKind := first(c.lastKind, "xray")
	c.stopLocked()
	c.kind = kind
	if e := c.startLocked(cfg, port); e != nil {
		if previous != nil {
			c.kind = previousKind
			if rollback := c.startLocked(previous, previousPort); rollback != nil {
				return fmt.Errorf("%v; previous core could not be restored: %v", e, rollback)
			}
		}
		return e
	}
	return nil
}
func xrayRules(routing Object, tag, key string) []any {
	rules := []any{}
	if !boolean(routing["enabled"]) {
		return rules
	}
	groups := obj(routing["groups_ui"])
	categories := []string{"block", "direct", "proxy"}
	if boolean(routing["direct_default"]) {
		categories = []string{"block", "proxy", "direct"}
	}
	for _, category := range categories {
		lines := []string{str(routing[category+"_rules"])}
		for _, v := range list(groups[category]) {
			g := obj(v)
			if enabled, ok := g["enabled"]; ok && !boolean(enabled) {
				continue
			}
			for _, v := range list(g["rules"]) {
				if m, ok := v.(map[string]any); ok {
					if enabled, exists := m["enabled"]; exists && !boolean(enabled) {
						continue
					}
					lines = append(lines, str(m["value"]))
				} else {
					lines = append(lines, str(v))
				}
			}
		}
		target, targetKey := tag, key
		if category == "block" {
			target = "blocked"
			targetKey = "outboundTag"
		}
		if category == "direct" {
			target = "direct"
			targetKey = "outboundTag"
		}
		domains, ips := []any{}, []any{}
		for _, line := range strings.Split(strings.Join(lines, "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			lower := strings.ToLower(line)
			_, _, cidrErr := net.ParseCIDR(line)
			if strings.HasPrefix(lower, "ip:") {
				ips = append(ips, strings.TrimSpace(line[3:]))
			} else if strings.HasPrefix(lower, "geoip:") || net.ParseIP(line) != nil || cidrErr == nil {
				ips = append(ips, line)
			} else {
				prefixed := false
				for _, prefix := range []string{"domain:", "full:", "geosite:", "regexp:", "ext:"} {
					if strings.HasPrefix(lower, prefix) {
						prefixed = true
						break
					}
				}
				if !prefixed {
					line = "domain:" + strings.TrimPrefix(line, "*.")
				}
				domains = append(domains, line)
			}
		}
		if len(domains) > 0 {
			rules = append(rules, Object{"type": "field", "domain": domains, targetKey: target})
		}
		if len(ips) > 0 {
			rules = append(rules, Object{"type": "field", "ip": ips, targetKey: target})
		}
	}
	return rules
}
func xrayConfig(server, data Object, port int) (Object, error) {
	routing := obj(data["routing"])
	if str(data["routing_engine"]) == "optirout" {
		routing = Object{}
	}
	inbound := Object{"protocol": "socks", "listen": "0.0.0.0", "port": port, "tag": "proxy0", "settings": Object{"auth": "noauth", "udp": true}, "sniffing": Object{"enabled": true, "destOverride": []any{"http", "tls", "quic"}, "routeOnly": true}}
	cfg := Object{"log": Object{"loglevel": "warning"}, "inbounds": []any{inbound}}
	tag, key := "", "outboundTag"
	outbounds := []any{}
	rt := Object{"domainStrategy": first(str(routing["domainStrategy"]), "IPIfNonMatch")}
	extra := []any{}
	if boolean(server["composite"]) {
		cd := obj(server["composite_data"])
		outbounds = list(cd["outbounds"])
		balancers := list(cd["balancers"])
		if len(balancers) > 0 {
			tag = str(obj(balancers[0])["tag"])
			key = "balancerTag"
			rt["balancers"] = balancers
		}
		if tag == "" {
			for _, v := range outbounds {
				ob := obj(v)
				if p := str(ob["protocol"]); p != "freedom" && p != "blackhole" && p != "dns" {
					tag = str(ob["tag"])
					break
				}
			}
		}
		for _, k := range []string{"observatory", "dns"} {
			if cd[k] != nil {
				cfg[k] = cd[k]
			}
		}
		rt["domainStrategy"] = first(str(cd["routing_domainStrategy"]), "IPIfNonMatch")
		rt["domainMatcher"] = first(str(cd["routing_domainMatcher"]), "hybrid")
		extra = list(cd["routing_extra_rules"])
	} else {
		if server["outbound"] == nil {
			return nil, fmt.Errorf("server requires a different core")
		}
		ob := clone(server["outbound"])
		tag = first(str(ob["tag"]), "proxy")
		ob["tag"] = tag
		outbounds = append(outbounds, ob)
	}
	if tag == "" {
		return nil, fmt.Errorf("configuration has no proxy target")
	}
	existing := map[string]bool{}
	for _, v := range outbounds {
		existing[str(obj(v)["tag"])] = true
	}
	if !existing["direct"] {
		outbounds = append(outbounds, Object{"protocol": "freedom", "tag": "direct"})
	}
	if !existing["blocked"] {
		outbounds = append(outbounds, Object{"protocol": "blackhole", "tag": "blocked"})
	}
	rules := xrayRules(routing, tag, key)
	fallback := Object{"type": "field", "inboundTag": []any{"proxy0"}, key: tag}
	if boolean(routing["enabled"]) && boolean(routing["direct_default"]) {
		fallback = Object{"type": "field", "inboundTag": []any{"proxy0"}, "outboundTag": "direct"}
	}
	rules = append(rules, fallback)
	rules = append(rules, extra...)
	rt["rules"] = rules
	cfg["routing"] = rt
	cfg["outbounds"] = outbounds
	return cfg, nil
}
