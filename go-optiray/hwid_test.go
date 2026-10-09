package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHWIDLegacyMigration(t *testing.T) {
	for _, tc := range []struct{ name, settings, legacy, want string }{
		{"text only", "", " legacy-id\n", "legacy-id"},
		{"settings wins", "restored-id", "stale-id", "restored-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.settings != "" {
				if err := writeObject(filepath.Join(dir, "subs.json"), Object{"hwid": tc.settings}); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "hwid.txt"), []byte(tc.legacy), 0600); err != nil {
				t.Fatal(err)
			}
			s, err := newStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if str(s.snapshot()["hwid"]) != tc.want {
				t.Fatal("wrong migrated identifier")
			}
			persisted, err := readObject(filepath.Join(dir, "subs.json"))
			if err != nil {
				t.Fatal(err)
			}
			if str(persisted["hwid"]) != tc.want {
				t.Fatal("migration not persisted")
			}
			if _, err := os.Stat(filepath.Join(dir, "hwid.txt")); !os.IsNotExist(err) {
				t.Fatal("legacy file remains", err)
			}
			again, err := newStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if str(again.snapshot()["hwid"]) != tc.want {
				t.Fatal("identifier changed on restart")
			}
		})
	}
}
func TestHWIDBackupRestoreSurvivesRestart(t *testing.T) {
	source, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := source.change(func(d Object) error { d["hwid"] = "subscription-device-id"; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][2]bool{{false, false}, {false, true}, {true, true}} {
		payload, _, err := source.backup(flags[0], flags[1])
		if err != nil {
			t.Fatal(err)
		}
		target, err := newStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		delete(payload, "optirout_config") // Routing runtime is tested separately; HWID uses the same settings payload.
		if err := target.restore(payload); err != nil {
			t.Fatal(err)
		}
		again, err := newStore(target.dir)
		if err != nil {
			t.Fatal(err)
		}
		if str(again.snapshot()["hwid"]) != "subscription-device-id" {
			t.Fatal("restored HWID lost on restart")
		}
		if _, err := os.Stat(filepath.Join(target.dir, "hwid.txt")); !os.IsNotExist(err) {
			t.Fatal("restore recreated text file", err)
		}
	}
}
func TestHWIDFailedLoadPreservesLegacy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "subs.json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hwid.txt"), []byte("legacy-id"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newStore(dir); err == nil {
		t.Fatal("corrupt settings accepted")
	}
	data, err := os.ReadFile(filepath.Join(dir, "hwid.txt"))
	if err != nil || string(data) != "legacy-id" {
		t.Fatal("legacy identity lost", err)
	}
}
