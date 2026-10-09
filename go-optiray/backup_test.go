package main

import (
	"net/url"
	"testing"
)

func TestRestoreWithoutRoutingPreservesRules(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.data["routing"] = Object{"block_rules": "keep.example"}
	payload := s.snapshot()
	delete(payload, "routing")
	payload["proxy_name"] = "Restored"
	if e = s.restore(payload); e != nil {
		t.Fatal(e)
	}
	d := s.snapshot()
	if str(obj(d["routing"])["block_rules"]) != "keep.example" || str(d["proxy_name"]) != "Restored" {
		t.Fatal("restore lost settings")
	}
}
func TestInvalidRestoreLeavesSettingsUntouched(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	before := str(s.snapshot()["hwid"])
	payload := s.snapshot()
	payload["routing"] = Object{"groups_ui": []any{}}
	if e = s.restore(payload); e == nil {
		t.Fatal("invalid routing accepted")
	}
	if str(s.snapshot()["hwid"]) != before {
		t.Fatal("failed restore changed settings")
	}
}
func TestSettingsValidationRollsBackAllChanges(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	original := str(s.snapshot()["hwid"])
	e = s.change(func(d Object) error {
		return applySettings(d, url.Values{"hwid": {"changed"}, "ping_timeout": {"999"}})
	})
	if e == nil || str(s.snapshot()["hwid"]) != original {
		t.Fatal("invalid form partially saved")
	}
}
func TestSettingsClientProfilePersists(t *testing.T) {
	s, e := newStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.change(func(d Object) error {
		return applySettings(d, url.Values{"subscription_client_profile": {"incy"}, "ping_real_concurrency": {"3"}})
	}); e != nil {
		t.Fatal(e)
	}
	again, e := newStore(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	if str(again.snapshot()["subscription_client_profile"]) != "incy" {
		t.Fatal("client setting lost")
	}
}
