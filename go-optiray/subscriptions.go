package main

import (
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
func decode64(s string) ([]byte, error) {
	s = strings.Join(strings.Fields(s), "")
	for _, enc := range []*base64.Encoding{base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding} {
		b, e := enc.DecodeString(s)
		if e == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid base64")
}
func streamSettings(sec, network, sni, host string, q url.Values) Object {
	stream := Object{"network": network, "security": sec}
	if sec == "reality" {
		stream["realitySettings"] = Object{"publicKey": q.Get("pbk"), "fingerprint": first(q.Get("fp"), "chrome"), "serverName": sni, "shortId": q.Get("sid"), "spiderX": first(q.Get("spx"), "/")}
	}
	if sec == "tls" {
		tls := Object{"serverName": first(sni, host)}
		if q.Get("fp") != "" {
			tls["fingerprint"] = q.Get("fp")
		}
		if pin := certPin(first(q.Get("pinnedPeerCertSha256"), q.Get("hash"))); pin != "" {
			tls["pinnedPeerCertSha256"] = pin
		}
		if q.Get("alpn") != "" {
			tls["alpn"] = strings.Split(q.Get("alpn"), ",")
		}
		stream["tlsSettings"] = tls
	}
	switch network {
	case "grpc":
		svc := first(q.Get("serviceName"), q.Get("path"))
		if svc != "" {
			stream["grpcSettings"] = Object{"mode": first(q.Get("mode"), "gun"), "serviceName": svc}
		}
	case "ws":
		stream["wsSettings"] = Object{"path": first(q.Get("path"), "/"), "headers": Object{"Host": first(q.Get("host"), sni, host)}}
	case "http", "h2":
		stream["network"] = "h2"
		hosts := []string{first(sni, host)}
		if q.Get("host") != "" {
			hosts = strings.Split(q.Get("host"), ",")
		}
		stream["httpSettings"] = Object{"path": first(q.Get("path"), "/"), "host": hosts}
	case "tcp":
		if q.Get("headerType") == "http" {
			stream["tcpSettings"] = Object{"header": Object{"type": "http", "request": Object{"path": []string{first(q.Get("path"), "/")}, "headers": Object{"Host": []string{first(sni, host)}}}}}
		}
	}
	return stream
}
func parseLink(link string) (Object, error) {
	if strings.HasPrefix(link, "vmess://") {
		raw, e := decode64(strings.Split(strings.TrimPrefix(link, "vmess://"), "#")[0])
		if e != nil {
			return nil, e
		}
		v := Object{}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		host := str(v["add"])
		port, _ := strconv.Atoi(str(v["port"]))
		if host == "" || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid VMess address")
		}
		name := first(str(v["ps"]), str(v["remarks"]), fmt.Sprintf("%s:%d", host, port))
		sec := "none"
		if str(v["tls"]) == "tls" {
			sec = "tls"
		}
		network := first(str(v["net"]), "tcp")
		q := url.Values{"path": {first(str(v["path"]), "/")}, "host": {str(v["host"])}, "mode": {first(str(v["type"]), "gun")}, "fp": {str(v["fp"])}}
		ob := Object{"protocol": "vmess", "tag": name, "streamSettings": streamSettings(sec, network, first(str(v["sni"]), str(v["host"])), host, q), "settings": Object{"vnext": []any{Object{"address": host, "port": port, "users": []any{Object{"id": str(v["id"]), "alterId": integer(v["aid"]), "security": first(str(v["scy"]), "auto")}}}}}}
		return serverFromOutbound(name, host, port, "vmess", network, sec, ob, "URI"), nil
	}
	if strings.HasPrefix(link, "ss://") {
		body := strings.TrimPrefix(link, "ss://")
		fragment := ""
		if i := strings.IndexByte(body, '#'); i >= 0 {
			fragment, _ = url.PathUnescape(body[i+1:])
			body = body[:i]
		}
		if !strings.Contains(body, "@") {
			decoded, e := decode64(body)
			if e != nil {
				return nil, e
			}
			body = string(decoded)
		}
		i := strings.LastIndexByte(body, '@')
		if i < 0 {
			return nil, fmt.Errorf("invalid Shadowsocks URI")
		}
		credentials := body[:i]
		if !strings.Contains(credentials, ":") {
			b, e := decode64(credentials)
			if e != nil {
				return nil, e
			}
			credentials = string(b)
		}
		parts := strings.SplitN(credentials, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Shadowsocks credentials")
		}
		u, e := url.Parse("ss://" + body[i+1:])
		if e != nil {
			return nil, e
		}
		port, _ := strconv.Atoi(u.Port())
		host := u.Hostname()
		if host == "" || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid Shadowsocks address")
		}
		name := first(fragment, fmt.Sprintf("%s:%d", host, port))
		ob := Object{"protocol": "shadowsocks", "tag": name, "streamSettings": Object{"network": "tcp"}, "settings": Object{"servers": []any{Object{"address": host, "port": port, "method": parts[0], "password": parts[1], "uot": false}}}}
		return serverFromOutbound(name, host, port, "shadowsocks", "tcp", "none", ob, "URI"), nil
	}
	u, e := url.Parse(strings.TrimSpace(link))
	if e != nil {
		return nil, e
	}
	host := u.Hostname()
	port, e := strconv.Atoi(u.Port())
	if e != nil || host == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid server address")
	}
	q := u.Query()
	name := first(u.Fragment, fmt.Sprintf("%s:%d", host, port))
	network := first(q.Get("type"), "tcp")
	sec := first(q.Get("security"), "none")
	sni := first(q.Get("sni"), q.Get("peer"))
	username, password := "", ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	protocol := u.Scheme
	ob := Object{"tag": name, "streamSettings": streamSettings(sec, network, sni, host, q)}
	switch protocol {
	case "vless":
		if username == "" {
			return nil, fmt.Errorf("missing UUID")
		}
		ob["settings"] = Object{"vnext": []any{Object{"address": host, "port": port, "users": []any{Object{"id": username, "encryption": first(q.Get("encryption"), "none"), "flow": q.Get("flow")}}}}}
	case "trojan":
		sec = first(q.Get("security"), "tls")
		ob["streamSettings"] = streamSettings(sec, network, sni, host, q)
		ob["settings"] = Object{"servers": []any{Object{"address": host, "port": port, "password": username}}}
	case "socks", "socks4", "socks5", "http", "https":
		if protocol == "http" || protocol == "https" {
			if username == "" || (u.Path != "" && u.Path != "/") {
				return nil, fmt.Errorf("invalid HTTP proxy URI")
			}
			if protocol == "https" {
				sec = "tls"
			}
			protocol = "http"
		} else {
			protocol = "socks"
		}
		network = "tcp"
		server := Object{"address": host, "port": port}
		if username != "" {
			server["users"] = []any{Object{"user": username, "pass": password}}
		}
		ob["settings"] = Object{"servers": []any{server}}
		ob["streamSettings"] = Object{"network": "tcp", "security": sec}
	case "wireguard":
		private := first(username, q.Get("privateKey"), q.Get("secretKey"))
		public := first(q.Get("publicKey"), q.Get("peer"))
		if private == "" || public == "" {
			return nil, fmt.Errorf("missing WireGuard keys")
		}
		mtu, _ := strconv.Atoi(first(q.Get("mtu"), "1420"))
		ob["settings"] = Object{"secretKey": private, "address": []any{first(q.Get("address"), "10.0.0.2/32")}, "mtu": mtu, "peers": []any{Object{"publicKey": public, "endpoint": u.Host}}}
		network = "tcp"
		sec = "none"
		ob["streamSettings"] = Object{"network": "tcp"}
	case "hysteria", "hy", "hysteria2", "hy2":
		if protocol == "hy" {
			protocol = "hysteria"
		}
		if protocol == "hy2" {
			protocol = "hysteria2"
		}
		srv := serverFromOutbound(name, host, port, protocol, "hysteria", "tls", nil, "URI")
		auth := username
		if password != "" {
			auth += ":" + password
		}
		srv["auth"] = first(auth, q.Get("auth"))
		srv["pinSHA256"] = first(q.Get("pinSHA256"), q.Get("pinsha256"))
		srv["up_mbps"] = integer(first(q.Get("upmbps"), q.Get("up")))
		srv["down_mbps"] = integer(first(q.Get("downmbps"), q.Get("down")))
		srv["sni"] = sni
		srv["insecure"] = q.Get("insecure") == "1"
		srv["obfs"] = q.Get("obfs")
		srv["obfs-password"] = q.Get("obfs-password")
		return srv, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %s", protocol)
	}
	ob["protocol"] = protocol
	srv := serverFromOutbound(name, host, port, protocol, network, sec, ob, "URI")
	srv["sni"] = sni
	return srv, nil
}
func serverFromOutbound(name, host string, port int, protocol, network, security string, ob Object, format string) Object {
	return Object{"name": name, "host": host, "port": port, "protocol": protocol, "transport": network, "security": security, "outbound": ob, "format": format, "sni": ""}
}
func outboundInfo(ob Object) (string, int, string) {
	settings := obj(ob["settings"])
	protocol := str(ob["protocol"])
	host, credential := "", ""
	port := 0
	entries := list(settings["servers"])
	if protocol == "vless" || protocol == "vmess" {
		entries = list(settings["vnext"])
	}
	if len(entries) > 0 {
		s := obj(entries[0])
		host = str(s["address"])
		port = integer(s["port"])
		credential = str(s["password"])
		users := list(s["users"])
		if len(users) > 0 {
			u := obj(users[0])
			if protocol == "vless" || protocol == "vmess" {
				credential = str(u["id"])
			}
		}
	}
	if host == "" {
		host = str(settings["address"])
		port = integer(settings["port"])
		credential = first(str(settings["password"]), str(settings["auth"]))
	}
	if protocol == "shadowsocks" && len(entries) > 0 {
		credential = str(obj(entries[0])["method"]) + ":" + str(obj(entries[0])["password"])
	}
	if protocol == "socks" || protocol == "http" {
		credential = ""
	}
	if protocol == "wireguard" {
		peers := list(settings["peers"])
		if len(peers) > 0 {
			peer := obj(peers[0])
			credential = str(peer["publicKey"])
			h, p, e := net.SplitHostPort(str(peer["endpoint"]))
			if e == nil {
				host = h
				port, _ = strconv.Atoi(p)
			}
		}
	}
	if protocol == "hysteria" || protocol == "hysteria2" {
		credential = first(str(obj(obj(ob["streamSettings"])["hysteriaSettings"])["auth"]), credential)
	}
	return host, port, credential
}
func serverID(s Object) string {
	_, _, cred := outboundInfo(obj(s["outbound"]))
	parts := []string{str(s["name"]), str(s["host"]), str(s["port"]), str(s["protocol"])}
	if boolean(s["composite"]) {
		parts = append(parts, "composite", str(s["candidates_count"]))
		for _, v := range list(obj(s["composite_data"])["outbounds"]) {
			if tag := str(obj(v)["tag"]); tag != "" {
				parts = append(parts, tag)
			}
		}
	} else {
		if s["outbound"] == nil {
			cred = str(s["auth"])
		}
		parts = append(parts, cred)
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", sum[:8])
}
func parseSubscription(content string) ([]any, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "[") && !strings.HasPrefix(content, "{") && !strings.Contains(content, "://") {
		if b, e := decode64(content); e == nil {
			content = string(b)
		}
	}
	var value any
	if json.Unmarshal([]byte(content), &value) == nil {
		configs := list(value)
		title := ""
		if _, ok := value.(map[string]any); ok {
			configs = []any{value}
			cfg := obj(value)
			title = first(str(cfg["profile_name"]), str(cfg["remarks"]), str(cfg["name"]))
		}
		servers := []any{}
		for _, v := range configs {
			cfg := obj(v)
			if cfg["outbounds"] == nil && cfg["config"] != nil {
				cfg = obj(cfg["config"])
			}
			if composite := compositeServer(cfg); composite != nil {
				servers = append(servers, composite)
				continue
			}
			for _, item := range list(cfg["outbounds"]) {
				ob := obj(item)
				protocol := str(ob["protocol"])
				if protocol == "freedom" || protocol == "blackhole" || protocol == "dns" {
					continue
				}
				host, port, _ := outboundInfo(ob)
				if host == "" || port < 1 {
					continue
				}
				name := first(str(cfg["remarks"]), str(cfg["name"]), str(cfg["comment"]))
				tag := str(ob["tag"])
				if name == "" && tag != "proxy" && tag != "direct" && tag != "blocked" {
					name = tag
				}
				name = first(name, fmt.Sprintf("%s:%d", host, port))
				stream := obj(ob["streamSettings"])
				srv := serverFromOutbound(name, host, port, protocol, first(str(stream["network"]), "tcp"), first(str(stream["security"]), "none"), normalizeOutbound(ob, len(servers)), "JSON")
				srv["sni"] = first(str(obj(stream["realitySettings"])["serverName"]), str(obj(stream["tlsSettings"])["serverName"]))
				if protocol == "tuic" || protocol == "anytls" {
					srv["outbound"] = nil
					srv["unsupported_reason"] = "Requires a separate " + protocol + " core"
				}
				srv["sid"] = serverID(srv)
				servers = append(servers, srv)
				break
			}
		}
		if len(servers) == 0 {
			return nil, title, fmt.Errorf("no supported servers in JSON")
		}
		uniqueServerIDs(servers)
		return servers, title, nil
	}
	servers := []any{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if s, e := parseLink(line); e == nil {
			s["sid"] = serverID(s)
			servers = append(servers, s)
		}
	}
	if len(servers) == 0 {
		return nil, "", fmt.Errorf("no supported servers in subscription")
	}
	return servers, "", nil
}
func subscriptionHeaders(profile string) http.Header {
	h := http.Header{}
	h.Set("Accept", "*/*")
	switch profile {
	case "optiray":
		h.Set("User-Agent", "OptiRay/"+version+" (Keenetic; Linux)")
		h.Set("x-device-os", "Linux")
		h.Set("x-device-model", "OptiRay / Keenetic")
	case "incy":
		h.Set("User-Agent", "Incy/3.7.0 (Linux; Android 16)")
		h.Set("x-device-os", "Android")
		h.Set("x-ver-os", "16")
		h.Set("x-device-model", "Incy Android")
	default:
		h.Set("User-Agent", "Happ/4.7.1 (Linux; Android 16)")
		h.Set("x-device-os", "Android")
		h.Set("x-ver-os", "16")
		h.Set("x-device-model", "Happ Android")
	}
	return h
}
func fetchSubscription(ctx context.Context, rawURL, profile, hwid string, useHWID bool) ([]any, string, error) {
	if useHWID {
		rawURL = strings.ReplaceAll(rawURL, "{hwid}", url.QueryEscape(hwid))
	}
	u, e := url.Parse(rawURL)
	if e != nil {
		return nil, "", e
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", fmt.Errorf("subscription URL must be HTTP(S)")
	}
	headers := subscriptionHeaders(profile)
	if useHWID && hwid != "" {
		q := u.Query()
		if !q.Has("hwid") {
			q.Set("hwid", hwid)
		}
		u.RawQuery = q.Encode()
		for _, key := range []string{"x-hwid", "hwid", "device-id"} {
			headers.Set(key, hwid)
		}
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return nil, "", e
	}
	req.Header = headers
	resp, e := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if e != nil {
		return nil, "", fmt.Errorf("subscription request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("subscription HTTP %d", resp.StatusCode)
	}
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, e := gzip.NewReader(reader)
		if e != nil {
			return nil, "", e
		}
		defer gz.Close()
		reader = gz
	}
	b, e := io.ReadAll(io.LimitReader(reader, (16<<20)+1))
	if e != nil {
		return nil, "", e
	}
	if len(b) > 16<<20 {
		return nil, "", fmt.Errorf("subscription too large")
	}
	servers, title, e := parseSubscription(string(b))
	if h := resp.Header.Get("Profile-Title"); h != "" {
		title = h
		if strings.HasPrefix(h, "base64:") {
			if decoded, e := decode64(strings.TrimPrefix(h, "base64:")); e == nil {
				title = string(decoded)
			}
		}
	}
	return servers, title, e
}
