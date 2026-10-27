package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Do not initialize defaults, recover journals or alter a running process here.
func checkInstallationFiles(ctx context.Context, dir string) error {
	for _, name := range []string{"subs.json", "auth.json", "routing/optirout.json", "routing/xray.json", "xray_config.json", "core_release_schedule.json", ".settings-transaction.json"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if _, err := readObject(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("повреждён или недоступен файл %s; восстановите его перед обновлением: %w", path, err)
		}
	}
	for _, name := range []string{"xray", "hysteria", "hysteria1"} {
		path := filepath.Join(dir, "bin", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		output, err := exec.CommandContext(checkCtx, path, "version").CombinedOutput()
		cancel()
		expected := "hysteria"
		if name == "xray" {
			expected = "xray"
		}
		if err != nil || !strings.Contains(strings.ToLower(string(output)), expected) {
			return fmt.Errorf("ядро %s не прошло проверку запуска; переустановите ядро перед обновлением", path)
		}
		if name == "xray" {
			config := filepath.Join(dir, "xray_config.json")
			if _, err := os.Stat(config); err == nil {
				checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				cmd := exec.CommandContext(checkCtx, path, "run", "-test", "-c", config)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+filepath.Join(dir, "bin"))
				err = cmd.Run()
				cancel()
				if err != nil {
					return fmt.Errorf("ядро %s отклонило %s; проверьте конфигурацию и GeoIP/GeoSite перед обновлением", path, config)
				}
			}
		}
	}
	return nil
}
