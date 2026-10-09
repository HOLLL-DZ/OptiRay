package main

import (
	"fmt"
	"net"
)

func hysteriaConfig(server Object, port int, listen string) (Object, error) {
	host := str(server["host"])
	remotePort := integer(server["port"])
	if host == "" || remotePort < 1 || remotePort > 65535 || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid Hysteria address")
	}
	kind := str(server["protocol"])
	tls := Object{"sni": first(str(server["sni"]), host), "insecure": boolean(server["insecure"])}
	if pin := str(server["pinSHA256"]); pin != "" {
		tls["pinSHA256"] = pin
	}
	cfg := Object{"server": net.JoinHostPort(host, fmt.Sprint(remotePort)), "tls": tls, "socks5": Object{"listen": net.JoinHostPort(listen, fmt.Sprint(port))}}
	if kind == "hysteria2" {
		cfg["auth"] = str(server["auth"])
		if obfs := str(server["obfs"]); obfs != "" {
			if obfs != "salamander" && obfs != "gecko" {
				return nil, fmt.Errorf("unsupported Hysteria2 obfuscation")
			}
			cfg["obfs"] = Object{"type": obfs, obfs: Object{"password": str(server["obfs-password"])}}
		}
	} else if kind == "hysteria" {
		cfg["auth_str"] = str(server["auth"])
		cfg["up_mbps"] = integer(server["up_mbps"])
		cfg["down_mbps"] = integer(server["down_mbps"])
		if integer(cfg["up_mbps"]) <= 0 {
			cfg["up_mbps"] = 100
		}
		if integer(cfg["down_mbps"]) <= 0 {
			cfg["down_mbps"] = 100
		}
		if obfs := str(server["obfs"]); obfs != "" {
			cfg["obfs"] = obfs
		}
	} else {
		return nil, fmt.Errorf("unknown Hysteria protocol")
	}
	return cfg, nil
}
func (c *CoreManager) ApplyServer(server, data Object, port int) error {
	if !optirayEnabled(data) {
		c.Stop()
		return nil
	}
	if server["outbound"] == nil && !boolean(server["composite"]) {
		kind := str(server["protocol"])
		cfg, e := hysteriaConfig(server, port, "0.0.0.0")
		if e != nil {
			return e
		}
		return c.ApplyKind(cfg, port, kind)
	}
	cfg, e := xrayConfig(server, data, port)
	if e != nil {
		return e
	}
	return c.Apply(cfg, port)
}

func supportedServer(s Object) bool {
	if boolean(s["composite"]) {
		return true
	}
	if s["outbound"] == nil {
		return str(s["protocol"]) == "hysteria" || str(s["protocol"]) == "hysteria2"
	}
	switch str(obj(s["outbound"])["protocol"]) {
	case "vless", "vmess", "trojan", "shadowsocks", "socks", "http", "wireguard", "hysteria", "hysteria2":
		return true
	}
	return false
}
