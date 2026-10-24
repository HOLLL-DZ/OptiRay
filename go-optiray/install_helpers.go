package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type lanCandidate struct{ name, address string }

func chooseLANAddress(items []lanCandidate) (string, error) {
	sort.SliceStable(items, func(i, j int) bool {
		rank := func(n string) int {
			if n == "br0" {
				return 0
			}
			if strings.HasPrefix(n, "br") {
				return 1
			}
			return 2
		}
		return rank(items[i].name) < rank(items[j].name)
	})
	for _, item := range items {
		ip := net.ParseIP(item.address)
		if (item.name == "br0" || strings.HasPrefix(item.name, "br")) && ip != nil && ip.To4() != nil && ip.IsPrivate() {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("не удалось определить IPv4-адрес домашнего моста роутера")
}
func routerLANAddress() (string, error) {
	interfaces, e := net.Interfaces()
	if e != nil {
		return "", e
	}
	items := []lanCandidate{}
	for _, iface := range interfaces {
		addrs, e := iface.Addrs()
		if e != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, e := net.ParseCIDR(addr.String())
			if e == nil {
				items = append(items, lanCandidate{iface.Name, ip.String()})
			}
		}
	}
	return chooseLANAddress(items)
}
func initializeInstallation(dir, address string) error {
	_, e := os.Stat(filepath.Join(dir, "subs.json"))
	fresh := os.IsNotExist(e)
	if e != nil && !fresh {
		return e
	}
	s, e := newStore(dir)
	if e != nil {
		return e
	}
	return s.change(func(d Object) error {
		if fresh {
			if ip := net.ParseIP(address); ip == nil || ip.To4() == nil {
				return fmt.Errorf("invalid LAN address")
			}
			d["proxy_host"] = address
			d["routing_url"] = "http://" + address + "/controlPanel/policy"
		}
		return nil
	})
}
func checkInstalledService(ctx context.Context, dir, address string) error {
	s := &Store{dir: dir}
	auth, e := s.auth()
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "GET", "http://"+address+"/api/health", nil)
	if e != nil {
		return e
	}
	if boolean(auth["enabled"]) {
		req.SetBasicAuth(str(auth["user"]), str(auth["pass"]))
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	response, e := (&http.Client{Transport: transport, Timeout: 5 * time.Second}).Do(req)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("HTTP health: %d", response.StatusCode)
	}
	var health Object
	if e = json.NewDecoder(response.Body).Decode(&health); e != nil {
		return e
	}
	if str(health["runtime"]) != "go" || str(health["version"]) != version || !boolean(health["ready"]) {
		return fmt.Errorf("OptiRay Go ещё не готов: проверьте log/manager.log и log/engine.log")
	}
	return nil
}

// Wait for slow cores and routing on constrained routers, with a bounded deadline.
func waitInstalledService(ctx context.Context, dir, address string, interval time.Duration) error {
	var last error
	for {
		if err := checkInstalledService(ctx, dir, address); err == nil {
			return nil
		} else {
			last = err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("service did not become ready: %w (last check: %v)", ctx.Err(), last)
		case <-timer.C:
		}
	}
}
