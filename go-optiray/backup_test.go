package main

import (
	"net/url"
	"testing"
)

func TestBackupExportPreferenceIndependentOfAutomaticCopies(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		for _, include := range []bool{false, true} {
			s, err := newStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			err = s.change(func(d Object) error {
				d["optirout_save_backups"] = automatic
				form := url.Values{}
				if include {
					form.Set("backup_include_optirout", "1")
				}
				return applySettings(d, form)
			})
			if err != nil {
				t.Fatal(err)
			}
			again, err := newStore(s.dir)
			if err != nil {
				t.Fatal(err)
			}
			d := again.snapshot()
			if boolean(d["optirout_save_backups"]) != automatic || boolean(d["backup_include_optirout"]) != include {
				t.Fatalf("preferences coupled: automatic=%v include=%v", automatic, include)
			}
		}
	}
}

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
