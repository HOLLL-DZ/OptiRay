package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Convert server outbounds, not the document's local routing or DNS policy.
func vlessJSONLinks(raw string) ([]string, error) {
	var root Object
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, fmt.Errorf("Некорректный JSON: %w", err)
	}
	outbounds := list(root["outbounds"])
	if str(root["protocol"]) == "vless" {
		outbounds = []any{root}
	}
	links := []string{}
	for _, entry := range outbounds {
		ob := obj(entry)
		if str(ob["protocol"]) != "vless" {
			continue
		}
		stream := obj(ob["streamSettings"])
		network := first(str(stream["network"]), "tcp")
		security := first(str(stream["security"]), "none")
		if network != "tcp" && network != "ws" && network != "grpc" && network != "http" && network != "h2" {
			return nil, fmt.Errorf("Транспорт %s пока нельзя преобразовать без потери настроек", network)
		}
		if security != "none" && security != "tls" && security != "reality" {
			return nil, fmt.Errorf("Не поддерживается security: %s", security)
		}
		for _, node := range list(obj(ob["settings"])["vnext"]) {
			server := obj(node)
			for _, userEntry := range list(server["users"]) {
				user := obj(userEntry)
				if str(user["id"]) == "" || str(server["address"]) == "" || integer(server["port"]) < 1 {
					return nil, fmt.Errorf("В JSON отсутствует UUID, адрес или порт")
				}
				q := url.Values{"encryption": {first(str(user["encryption"]), "none")}, "type": {network}, "security": {security}}
				if flow := str(user["flow"]); flow != "" {
					q.Set("flow", flow)
				}
				tls := obj(stream["tlsSettings"])
				if security == "reality" {
					tls = obj(stream["realitySettings"])
				}
				for field, key := range map[string]string{"serverName": "sni", "fingerprint": "fp", "publicKey": "pbk", "shortId": "sid", "spiderX": "spx"} {
					if value := str(tls[field]); value != "" {
						q.Set(key, value)
					}
				}
				if boolean(tls["allowInsecure"]) {
					return nil, fmt.Errorf("allowInsecure нельзя безопасно перенести в ссылку")
				}
				if alpn := list(tls["alpn"]); len(alpn) > 0 {
					values := []string{}
					for _, v := range alpn {
						values = append(values, str(v))
					}
					q.Set("alpn", strings.Join(values, ","))
				}
				switch network {
				case "grpc":
					g := obj(stream["grpcSettings"])
					q.Set("serviceName", str(g["serviceName"]))
					if boolean(g["multiMode"]) {
						q.Set("mode", "multi")
					}
				case "ws":
					w := obj(stream["wsSettings"])
					q.Set("path", first(str(w["path"]), "/"))
					if host := str(obj(w["headers"])["Host"]); host != "" {
						q.Set("host", host)
					}
				case "http", "h2":
					h := obj(stream["httpSettings"])
					q.Set("path", first(str(h["path"]), "/"))
					hosts := []string{}
					for _, v := range list(h["host"]) {
						hosts = append(hosts, str(v))
					}
					if len(hosts) > 0 {
						q.Set("host", strings.Join(hosts, ","))
					}
				case "tcp":
					if str(obj(obj(stream["tcpSettings"])["header"])["type"]) == "http" {
						return nil, fmt.Errorf("TCP HTTP header требует отдельного преобразования")
					}
				}
				name := first(str(ob["remarks"]), str(ob["tag"]), str(root["remarks"]), str(server["address"]))
				u := url.URL{Scheme: "vless", User: url.User(str(user["id"])), Host: net.JoinHostPort(str(server["address"]), strconv.Itoa(integer(server["port"]))), RawQuery: q.Encode(), Fragment: name}
				link := u.String()
				if _, err := parseLink(link); err != nil {
					return nil, err
				}
				links = append(links, link)
			}
		}
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("В JSON не найдены VLESS-серверы")
	}
	return links, nil
}

func expandJSONConfigurations(d Object) error {
	subs := list(d["subscriptions"])
	maxID := 0
	for _, v := range subs {
		if id := integer(obj(v)["id"]); id > maxID {
			maxID = id
		}
	}
	next := []any{}
	for _, v := range subs {
		sub := obj(v)
		raw := strings.TrimSpace(str(sub["url"]))
		if !strings.HasPrefix(raw, "{") {
			next = append(next, sub)
			continue
		}
		links, err := vlessJSONLinks(raw)
		if err != nil {
			return err
		}
		for i, link := range links {
			copy := clone(sub)
			if i > 0 {
				maxID++
				copy["id"] = maxID
			}
			copy["url"] = link
			next = append(next, copy)
		}
	}
	d["subscriptions"] = next
	return nil
}
