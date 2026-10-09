//go:build !linux

package main

import "fmt"

func legacyRunning() bool                      { return false }
func stopLegacyInstallation() error            { return fmt.Errorf("legacy migration requires Linux") }
func startLegacyInstallation(dir string) error { return fmt.Errorf("legacy migration requires Linux") }

func captureLegacyAuthentication(dir string) error { return nil }
