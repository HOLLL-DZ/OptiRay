package main

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

func legacyProcessRole(exe, cwd string, args []string, engineOwned bool) string {
	resolve := func(p string) string {
		if !path.IsAbs(p) {
			p = path.Join(cwd, p)
		}
		return path.Clean(p)
	}
	contains := func(path string) bool {
		for _, arg := range args[1:] {
			if resolve(arg) == path {
				return true
			}
		}
		return false
	}
	if len(args) == 0 {
		return ""
	}
	if strings.HasPrefix(path.Base(exe), "python") && contains("/opt/etc/optiray/app.py") {
		return "manager"
	}
	if (path.Base(exe) == "busybox" || path.Base(exe) == "sh" || path.Base(exe) == "bash") && contains("/opt/etc/optiray/watchdog.sh") {
		return "watchdog"
	}
	for i, arg := range args {
		if (arg == "-c" || arg == "--config") && i+1 < len(args) {
			config := resolve(args[i+1])
			if exe == "/opt/bin/xray" && config == "/opt/etc/optiray/xray_config.json" || exe == "/opt/bin/hysteria" && config == "/opt/etc/optiray/hysteria_config.yaml" {
				return "core"
			}
		}
	}
	if exe == "/opt/etc/optiray/bin/optiroutd" && engineOwned {
		return "routing"
	}
	return ""
}

func authenticationFromEnvironment(raw []byte) Object {
	values := map[string]string{}
	for _, entry := range strings.Split(string(raw), "\x00") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			values[parts[0]] = parts[1]
		}
	}
	user := strings.TrimSpace(values["OPTIRAY_USER"])
	if user == "" {
		return nil
	}
	return Object{"enabled": true, "user": user, "pass": first(values["OPTIRAY_PASS"], "admin")}
}

// Read only literal assignments; never execute a legacy init script to extract secrets.
func legacyAuthenticationFromService(text string) (Object, error) {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name != "OPTIRAY_USER" && name != "OPTIRAY_PASS" {
			continue
		}
		value := strings.TrimSpace(parts[1])
		var decoded string
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
			decoded = value[1 : len(value)-1]
			if strings.Contains(decoded, "'") {
				return nil, fmt.Errorf("unsupported legacy auth assignment")
			}
		} else if strings.HasPrefix(value, `"`) {
			if strings.ContainsAny(value, "$`\n") {
				return nil, fmt.Errorf("legacy authentication uses shell expansion; migrate while the legacy service is running")
			}
			var e error
			decoded, e = strconv.Unquote(value)
			if e != nil {
				return nil, fmt.Errorf("unsupported legacy auth quoting")
			}
		} else {
			if index := strings.Index(value, " #"); index >= 0 {
				value = value[:index]
			}
			if strings.ContainsAny(value, "$` ;\t\\<>&|") {
				return nil, fmt.Errorf("unsupported legacy authentication expression")
			}
			decoded = value
		}
		values[name] = decoded
	}
	if strings.TrimSpace(values["OPTIRAY_USER"]) == "" {
		return nil, nil
	}
	return Object{"enabled": true, "user": values["OPTIRAY_USER"], "pass": first(values["OPTIRAY_PASS"], "admin")}, nil
}
