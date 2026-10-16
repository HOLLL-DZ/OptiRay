package main

import (
	"context"
	"net/url"
	"testing"
)

func TestDeleteLastConfigurationRemovesOnlyItsGroup(t *testing.T) {
	for _, count := range []int{1, 2} {
		store, err := newStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		servers := []any{Object{"sid": "a"}}
		links := []any{"vless://a@example.com:443"}
		if count == 2 {
			servers = append(servers, Object{"sid": "b"})
			links = append(links, "vless://b@example.com:443")
		}
		err = store.change(func(d Object) error {
			d["subscriptions"] = []any{Object{"id": 1, "standalone_group": true, "servers": servers, "config_links": links}, Object{"id": 2, "servers": []any{}}}
			d["selected"] = ""
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		app := &App{store: store}
		err = app.manageSubscriptions(context.Background(), url.Values{"action": {"delete_config"}, "sub_id": {"1"}, "config_index": {"0"}})
		if err != nil {
			t.Fatal(err)
		}
		subs := list(store.snapshot()["subscriptions"])
		if len(subs) != count {
			t.Fatalf("unexpected groups: %d", len(subs))
		}
		if integer(obj(subs[len(subs)-1])["id"]) != 2 {
			t.Fatal("ordinary empty subscription removed")
		}
	}
}
