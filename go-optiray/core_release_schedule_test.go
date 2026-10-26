package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type coreScheduleFixture struct{ requests int }

func (f *coreScheduleFixture) RoundTrip(r *http.Request) (*http.Response, error) {
	f.requests++
	tag := "v26.10.21"
	if strings.Contains(r.URL.Path, "/apernet/hysteria/") {
		tag = "app/v2.13.0"
	}
	body := `{"tag_name":"` + tag + `","prerelease":false}`
	if !strings.HasSuffix(r.URL.Path, "/latest") {
		body = "[" + body + "]"
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}
func TestCoreReleaseScheduleTimes(t *testing.T) {
	for _, tc := range []struct {
		stamp string
		due   bool
	}{
		{"2026-10-26T01:59:00Z", false}, {"2026-10-26T02:00:00Z", true},
		{"2026-10-26T02:00:59Z", true}, {"2026-10-26T02:01:00Z", false},
		{"2026-10-26T11:00:00Z", true}, {"2026-10-26T11:01:00Z", false},
	} {
		now, _ := time.Parse(time.RFC3339, tc.stamp)
		_, due := coreReleaseSlot(now)
		if due != tc.due {
			t.Fatal(tc.stamp, due)
		}
	}
}
func TestCoreReleaseScheduleAtMostTwiceDailyAcrossRestart(t *testing.T) {
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	fixture := &coreScheduleFixture{}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.change(func(d Object) error { d["xray_auto_check"] = true; d["hysteria_auto_check"] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, _ := time.Parse(time.RFC3339, "2026-10-26T05:00:00+03:00")
	s.scheduledCoreReleaseCheck(ctx, first)
	if fixture.requests != 4 {
		t.Fatal("morning check missing", fixture.requests)
	}
	s.scheduledCoreReleaseCheck(ctx, first.Add(30*time.Second))
	s, err = newStore(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	s.scheduledCoreReleaseCheck(ctx, first.Add(45*time.Second))
	if fixture.requests != 4 {
		t.Fatal("duplicate check after restart")
	}
	s.scheduledCoreReleaseCheck(ctx, first.Add(2*time.Hour))
	if fixture.requests != 4 {
		t.Fatal("unscheduled check")
	}
	s.scheduledCoreReleaseCheck(ctx, first.Add(9*time.Hour))
	if fixture.requests != 8 {
		t.Fatal("afternoon check missing")
	}
	s.scheduledCoreReleaseCheck(ctx, first.Add(9*time.Hour+30*time.Second))
	if fixture.requests != 8 {
		t.Fatal("duplicate afternoon check")
	}
	s.scheduledCoreReleaseCheck(ctx, first.Add(24*time.Hour))
	if fixture.requests != 12 {
		t.Fatal("next day check missing")
	}
}

func TestCoreAutomaticChecksAreOptionalPerCore(t *testing.T) {
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	fixture := &coreScheduleFixture{}
	http.DefaultClient = &http.Client{Transport: fixture}
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now, _ := time.Parse(time.RFC3339, "2026-10-26T05:00:00+03:00")
	s.scheduledCoreReleaseCheck(context.Background(), now)
	if fixture.requests != 0 {
		t.Fatal("automatic checks enabled by default")
	}
	if err = s.change(func(d Object) error { d["xray_auto_check"] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	s.scheduledCoreReleaseCheck(context.Background(), now)
	if fixture.requests != 2 {
		t.Fatal("Xray-only preference not respected")
	}
	if err = s.change(func(d Object) error { d["hysteria_auto_check"] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	s.scheduledCoreReleaseCheck(context.Background(), now)
	if fixture.requests != 4 {
		t.Fatal("disabled Hysteria consumed its slot")
	}
	if err = s.change(func(d Object) error { d["xray_auto_check"] = false; d["hysteria_auto_check"] = false; return nil }); err != nil {
		t.Fatal(err)
	}
	s.scheduledCoreReleaseCheck(context.Background(), now.Add(9*time.Hour))
	if fixture.requests != 4 {
		t.Fatal("disabled check still ran")
	}
}
