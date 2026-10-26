package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

var coreReleaseScheduleZone = time.FixedZone("MSK", 3*60*60)

func coreReleaseSlot(now time.Time) (string, bool) {
	local := now.In(coreReleaseScheduleZone)
	if local.Minute() != 0 || (local.Hour() != 5 && local.Hour() != 14) {
		return "", false
	}
	return local.Format("2006-01-02T15"), true
}

// Claim each daily time slot before the network request, including failed attempts.
// Persisting the claim prevents duplicate checks on a manager restart in that minute.
func (s *Store) claimCoreReleaseSlot(kind, slot string) (bool, error) {
	s.releaseMu.Lock()
	defer s.releaseMu.Unlock()
	path := filepath.Join(s.dir, "core_release_schedule.json")
	state, err := readObject(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if state == nil {
		state = Object{}
	}
	key := kind + "_" + slot[len(slot)-2:]
	if str(state[key]) >= slot {
		return false, nil
	}
	state[key] = slot
	if err = writeObject(path, state); err != nil {
		return false, err
	}
	return true, nil
}
func (s *Store) scheduledCoreReleaseCheck(ctx context.Context, now time.Time) {
	slot, due := coreReleaseSlot(now)
	if !due {
		return
	}
	for _, kind := range []string{"xray", "hysteria"} {
		if !boolean(s.snapshot()[kind+"_auto_check"]) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		claimed, err := s.claimCoreReleaseSlot(kind, slot)
		if err != nil {
			log.Printf("Core update schedule: %v", err)
			continue
		}
		if !claimed {
			continue
		}
		if kind == "xray" {
			s.xrayReleases(ctx, true)
		} else {
			s.hysteriaReleases(ctx, true)
		}
	}
}
func (s *Store) superviseCoreReleaseSchedule(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.scheduledCoreReleaseCheck(ctx, now)
		}
	}
}
