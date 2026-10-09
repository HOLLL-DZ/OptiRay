package main

import (
	"context"
	"errors"
	"testing"
)

func autoData() Object {
	return Object{"subscriptions": []any{Object{"id": 1, "servers": []any{Object{"sid": "first", "outbound": Object{"protocol": "vless"}}, Object{"sid": "second", "outbound": Object{"protocol": "vless"}}, Object{"sid": "unsupported"}}}}, "favorites": []any{"second"}, "auto_select_source": "all"}
}
func TestAutoSelectUsesHTTPAndRejectsFailedSamples(t *testing.T) {
	d := autoData()
	best, warning, pings, e := chooseServer(context.Background(), d, 1, func(_ context.Context, s Object, kind string) int {
		if kind == "tcp" {
			if str(s["sid"]) == "first" {
				return 5
			}
			return 50
		}
		if str(s["sid"]) == "first" {
			return -1
		}
		return 100
	})
	if e != nil || str(best["sid"]) != "second" || warning != "" {
		t.Fatalf("%v %s %v", best, warning, e)
	}
	if str(obj(pings["second"])["text"]) != "100 ms" {
		t.Fatal(pings)
	}
}
func TestAutoSelectFavoritesOnly(t *testing.T) {
	d := autoData()
	d["auto_select_source"] = "favorites"
	best, _, _, e := chooseServer(context.Background(), d, 1, func(_ context.Context, s Object, _ string) int {
		if str(s["sid"]) != "second" {
			t.Error("tested nonfavorite server")
		}
		return 1
	})
	if e != nil || str(best["sid"]) != "second" {
		t.Fatal(e)
	}
}
func TestAutoSelectStableTiePreservesSourceOrder(t *testing.T) {
	best, _, _, e := chooseServer(context.Background(), autoData(), 1, func(context.Context, Object, string) int { return 0 })
	if e != nil || str(best["sid"]) != "first" {
		t.Fatal("tie reordered servers")
	}
}
func TestAutoSelectAllFailedDoesNotPickServer(t *testing.T) {
	best, _, _, e := chooseServer(context.Background(), autoData(), 1, func(context.Context, Object, string) int { return -1 })
	if e == nil || best != nil {
		t.Fatal("failed candidate selected")
	}
}
func TestAutoSelectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	best, _, _, e := chooseServer(ctx, autoData(), 1, func(context.Context, Object, string) int { t.Error("canceled probe executed"); return 5 })
	if !errors.Is(e, context.Canceled) || best != nil {
		t.Fatal(e)
	}
}
func TestAutoSelectTCPFallbackWarns(t *testing.T) {
	best, warning, _, e := chooseServer(context.Background(), autoData(), 1, func(_ context.Context, s Object, kind string) int {
		if kind == "real" {
			return -1
		}
		if str(s["sid"]) == "second" {
			return 10
		}
		return 100
	})
	if e != nil || str(best["sid"]) != "second" || warning == "" {
		t.Fatal("missing TCP fallback warning")
	}
}
