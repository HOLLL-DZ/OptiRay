package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) auth() (Object, error) {
	auth, e := readObject(filepath.Join(s.dir, "auth.json"))
	if os.IsNotExist(e) {
		auth = Object{"enabled": false, "user": "admin", "pass": "admin"}
		if user := strings.TrimSpace(os.Getenv("OPTIRAY_USER")); user != "" {
			auth["enabled"] = true
			auth["user"] = user
			auth["pass"] = first(os.Getenv("OPTIRAY_PASS"), "admin")
		}
		return auth, nil
	}
	if e != nil {
		return nil, e
	}
	if _, ok := auth["enabled"].(bool); !ok {
		return nil, fmt.Errorf("invalid authentication settings")
	}
	if str(auth["user"]) == "" || str(auth["pass"]) == "" {
		return nil, fmt.Errorf("empty authentication credentials")
	}
	return auth, nil
}
func authFromForm(previous Object, form url.Values) (Object, error) {
	next := clone(previous)
	next["enabled"] = form.Has("auth_enabled")
	if form.Has("auth_user") {
		user := strings.TrimSpace(form.Get("auth_user"))
		if user == "" || strings.ContainsAny(user, "\r\n:") {
			return nil, fmt.Errorf("invalid authentication user")
		}
		next["user"] = user
	}
	if password := strings.TrimSpace(form.Get("auth_pass")); password != "" {
		next["pass"] = password
	}
	return next, nil
}
