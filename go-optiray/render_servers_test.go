package main

import (
	"strings"
	"testing"
)

func TestServerRenderingKeepsSubscriptionsAndSourceOrder(t *testing.T) {
	d := Object{"subscriptions": []any{Object{"id": 1, "name": "First", "servers": []any{Object{"sid": "normal", "name": "Normal"}, Object{"sid": "active", "name": "Active"}}}, Object{"id": 2, "name": "Second", "servers": []any{Object{"sid": "fav", "name": "Favorite"}}}}, "selected": "active", "favorites": []any{"fav"}}
	out := renderServerCards(d)
	if strings.Index(out, `id="sub-section-1"`) > strings.Index(out, `id="sub-section-2"`) {
		t.Fatal("subscriptions reordered")
	}
	if strings.Index(out, `data-sid="active"`) > strings.Index(out, `data-sid="normal"`) {
		t.Fatal("active server not pinned")
	}
	if !strings.Contains(out, `data-sid="active" data-source-order="1"`) {
		t.Fatal("source order lost")
	}
}
func TestRenderingEscapesServerAndSubscriptionValues(t *testing.T) {
	d := Object{"subscriptions": []any{Object{"id": 1, "name": `<script>alert(1)</script>`, "color": `red; background:url(bad)`, "url": `" onfocus="bad`, "servers": []any{Object{"sid": "safe", "name": `<img src=x onerror=bad>`}}}}}
	rows, _ := renderSubscriptionInputs(d)
	cards := renderServerCards(d)
	if strings.Contains(rows, "<script>") || strings.Contains(rows, `value="" onfocus=`) || strings.Contains(cards, "<img src=x") || strings.Contains(cards, "background:url(bad)") {
		t.Fatal("unescaped external values")
	}
}
func TestServerFlagByEmojiAndName(t *testing.T) {
	for _, name := range []string{"🇩🇪 Germany", "Германия", "Germany"} {
		if !strings.Contains(serverFlag(name), "/api/flag/de") {
			t.Fatalf("flag missing for %s", name)
		}
	}
}
