package main

import (
	"context"
	"errors"
	"testing"
)

func refreshStore(t *testing.T) *Store {
	t.Helper()
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.data["subscriptions"] = []any{Object{"id": 1, "url": "https://one.invalid", "servers": []any{Object{"sid": "old", "name": "Same name"}}}, Object{"id": 2, "url": "https://two.invalid", "servers": []any{Object{"sid": "keep", "name": "Same name"}}}}
	s.data["favorites"] = []any{"old", "keep"}
	s.data["selected"] = "old"
	return s
}
func TestRefreshPreservesFailedSubscriptionAndRecoversWithinSameSubscription(t *testing.T) {
	s := refreshStore(t)
	e := s.refresh(context.Background(), func(_ context.Context, u, _, _ string, _ bool) ([]any, string, error) {
		if u == "https://two.invalid" {
			return nil, "", errors.New("network failure")
		}
		return []any{Object{"sid": "new", "name": "Same name"}}, "New title", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	d := s.snapshot()
	if str(d["selected"]) != "new" {
		t.Fatal("selected server not recovered")
	}
	subs := list(d["subscriptions"])
	if integer(obj(subs[0])["id"]) != 1 || !boolean(obj(subs[1])["stale"]) || str(obj(list(obj(subs[1])["servers"])[0])["sid"]) != "keep" {
		t.Fatal("failed subscription lost or subscriptions reordered")
	}
	if len(list(d["favorites"])) != 1 || str(list(d["favorites"])[0]) != "keep" {
		t.Fatal("favorites not reconciled")
	}
}
func TestRefreshDoesNotOverwriteConcurrentURLOrFavoriteChanges(t *testing.T) {
	s := refreshStore(t)
	changed := false
	e := s.refresh(context.Background(), func(_ context.Context, _, _, _ string, _ bool) ([]any, string, error) {
		if !changed {
			changed = true
			if e := s.change(func(d Object) error {
				obj(list(d["subscriptions"])[0])["url"] = "https://replacement.invalid"
				d["favorites"] = []any{"keep"}
				return nil
			}); e != nil {
				t.Fatal(e)
			}
		}
		return []any{Object{"sid": "keep", "name": "Same name"}}, "", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	d := s.snapshot()
	first := obj(list(d["subscriptions"])[0])
	if str(first["url"]) != "https://replacement.invalid" || str(obj(list(first["servers"])[0])["sid"]) != "old" {
		t.Fatal("stale response overwrote changed URL")
	}
	if len(list(d["favorites"])) != 1 {
		t.Fatal("concurrent favorites lost")
	}
}
func TestCanceledRefreshLeavesSettingsUntouched(t *testing.T) {
	s := refreshStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := s.refresh(ctx, fetchSubscription); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if str(s.snapshot()["selected"]) != "old" {
		t.Fatal("canceled refresh changed settings")
	}
}
