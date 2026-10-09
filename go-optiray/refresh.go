package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type subscriptionFetch func(context.Context, string, string, string, bool) ([]any, string, error)
type refreshResult struct {
	id      int
	url     string
	hwid    string
	profile string
	useHWID bool
	servers []any
	title   string
	err     error
}

// Network requests run outside the settings lock. Results are applied to the
// current document so favorites and subscription order changed meanwhile survive.
func (s *Store) refresh(ctx context.Context, fetch subscriptionFetch) error {
	return s.refreshApply(ctx, fetch, nil)
}
func (s *Store) refreshApply(ctx context.Context, fetch subscriptionFetch, apply func(Object) error) error {
	before := s.snapshot()
	results := []refreshResult{}
	for _, v := range list(before["subscriptions"]) {
		sub := obj(v)
		r := refreshResult{id: integer(sub["id"]), url: str(sub["url"]), hwid: str(before["hwid"]), profile: str(before["subscription_client_profile"]), useHWID: boolean(sub["use_hwid"])}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if strings.TrimSpace(r.url) != "" {
			r.servers, r.title, r.err = fetch(ctx, r.url, r.profile, r.hwid, r.useHWID)
		} else {
			r.servers = []any{}
		}
		results = append(results, r)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.change(func(d Object) error {
		oldServer, oldSub := findServer(d, str(d["selected"]), "")
		oldName := str(oldServer["name"])
		oldID := integer(oldSub["id"])
		for _, v := range list(d["subscriptions"]) {
			sub := obj(v)
			for _, r := range results {
				if integer(sub["id"]) != r.id || str(sub["url"]) != r.url || boolean(sub["use_hwid"]) != r.useHWID || str(d["hwid"]) != r.hwid || str(d["subscription_client_profile"]) != r.profile {
					continue
				}
				if r.err != nil {
					sub["stale"] = true
					continue
				}
				uniqueServerIDs(r.servers)
				sub["servers"] = r.servers
				delete(sub, "stale")
				if r.url != "" {
					sub["last_update"] = time.Now().Format("02.01.2006 15:04")
				}
				if r.title != "" {
					sub["name"] = r.title
				}
			}
		}
		valid := map[string]bool{}
		for _, v := range list(d["subscriptions"]) {
			for _, x := range list(obj(v)["servers"]) {
				valid[str(obj(x)["sid"])] = true
			}
		}
		favs := []any{}
		seen := map[string]bool{}
		for _, v := range list(d["favorites"]) {
			id := str(v)
			if valid[id] && !seen[id] {
				favs = append(favs, id)
				seen[id] = true
			}
		}
		d["favorites"] = favs
		selected := str(d["selected"])
		if selected != "" && !valid[selected] {
			d["selected"] = ""
			for _, v := range list(d["subscriptions"]) {
				sub := obj(v)
				if integer(sub["id"]) != oldID {
					continue
				}
				for _, x := range list(sub["servers"]) {
					srv := obj(x)
					if oldName != "" && str(srv["name"]) == oldName {
						d["selected"] = str(srv["sid"])
						break
					}
				}
			}
		}
		if apply != nil {
			return apply(d)
		}
		return nil
	})
}
func uniqueServerIDs(servers []any) {
	groups := map[string][]Object{}
	for _, v := range servers {
		s := obj(v)
		id := str(s["sid"])
		if id == "" {
			id = serverID(s)
		}
		groups[id] = append(groups[id], s)
	}
	for id, group := range groups {
		sort.SliceStable(group, func(i, j int) bool { return canonicalServer(group[i]) < canonicalServer(group[j]) })
		for i, s := range group {
			if i == 0 {
				s["sid"] = id
			} else {
				s["sid"] = fmt.Sprintf("%s-%d", id, i)
			}
		}
	}
}
