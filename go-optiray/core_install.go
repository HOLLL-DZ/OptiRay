package main

import (
	"archive/zip"
	"context"
	"debug/elf"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func coreArchitecture(kind, arch string) (string, error) {
	if kind == "xray" {
		switch arch {
		case "arm64":
			return "arm64-v8a", nil
		case "arm":
			return "arm32-v7a", nil
		case "mipsle":
			return "mips32le", nil
		case "mips":
			return "mips32", nil
		case "amd64":
			return "64", nil
		case "386":
			return "32", nil
		}
	} else {
		switch arch {
		case "arm64", "arm", "mipsle", "mips", "amd64", "386":
			return arch, nil
		}
	}
	return "", fmt.Errorf("unsupported core architecture %s", arch)
}
func validateELF(path, arch string) error {
	f, e := elf.Open(path)
	if e != nil {
		return fmt.Errorf("download is not a valid ELF binary")
	}
	defer f.Close()
	machine := map[string]elf.Machine{"arm64": elf.EM_AARCH64, "arm": elf.EM_ARM, "mipsle": elf.EM_MIPS, "mips": elf.EM_MIPS, "amd64": elf.EM_X86_64, "386": elf.EM_386}[arch]
	if f.Machine != machine {
		return fmt.Errorf("downloaded core architecture mismatch")
	}
	expectedClass := elf.ELFCLASS32
	if arch == "amd64" || arch == "arm64" {
		expectedClass = elf.ELFCLASS64
	}
	if f.Class != expectedClass {
		return fmt.Errorf("core word size mismatch")
	}
	if arch != "mips" && f.Data != elf.ELFDATA2LSB {
		return fmt.Errorf("core byte order mismatch")
	}
	if arch == "mipsle" && f.Data != elf.ELFDATA2LSB {
		return fmt.Errorf("MIPSel core byte order mismatch")
	}
	return nil
}
func downloadCore(ctx context.Context, address, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", address, nil)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "OptiRay/"+version)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return fmt.Errorf("core download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("core download HTTP %d", resp.StatusCode)
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	n, e := io.Copy(f, io.LimitReader(resp.Body, (128<<20)+1))
	if e == nil && n > 128<<20 {
		e = fmt.Errorf("core download exceeds size limit")
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
func (a *App) coreAction(ctx context.Context, kind, action, requestedVersion string) error {
	if kind != "xray" && kind != "hysteria2" {
		return fmt.Errorf("unknown core")
	}
	root := filepath.Join(a.store.dir, "bin")
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	name := kind
	if kind == "hysteria2" {
		name = "hysteria"
	}
	target := filepath.Join(root, name)
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	if a.core != nil {
		configured := a.core.bin
		if kind == "hysteria2" {
			configured = a.core.hysteriaBin
		}
		absolute, _ := filepath.Abs(configured)
		expected, _ := filepath.Abs(target)
		if absolute != expected {
			return fmt.Errorf("core management is limited to this installation's bin directory")
		}
	}
	if action == "uninstall" {
		d := a.store.snapshot()
		active := a.core != nil && a.core.Running() && first(a.core.kind, "xray") == kind
		if active {
			a.core.Stop()
		}
		removed := target + ".removed"
		if e := os.Rename(target, removed); e != nil && !os.IsNotExist(e) {
			if active {
				_ = a.rollbackRuntime(d, nil)
			}
			return e
		}
		if active {
			if e := a.store.change(func(next Object) error { next["selected"] = ""; return nil }); e != nil {
				_ = os.Rename(removed, target)
				if rollback := a.rollbackRuntime(d, nil); rollback != nil {
					return fmt.Errorf("remove failed: %v; rollback: %v", e, rollback)
				}
				return e
			}
		}
		if e := os.Remove(removed); e != nil && !os.IsNotExist(e) {
			return e
		}
		return nil
	}
	if action != "install" {
		return fmt.Errorf("unknown core action")
	}
	requestedVersion = strings.TrimPrefix(requestedVersion, "v")
	if kind == "hysteria2" && requestedVersion == "" {
		requestedVersion = "2.6.2"
	}
	if !coreVersionPattern.MatchString(requestedVersion) {
		return fmt.Errorf("invalid core version")
	}
	arch, e := coreArchitecture(kind, runtime.GOARCH)
	if e != nil {
		return e
	}
	temp, e := os.MkdirTemp(root, ".core-install-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	download := filepath.Join(temp, "download")
	address := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/v%s/Xray-linux-%s.zip", requestedVersion, arch)
	if kind == "hysteria2" {
		address = fmt.Sprintf("https://github.com/apernet/hysteria/releases/download/app%%2Fv%s/hysteria-linux-%s", requestedVersion, arch)
	}
	if e = downloadCore(ctx, address, download); e != nil {
		return e
	}
	binary := filepath.Join(temp, name)
	files := []string{name}
	if kind == "xray" {
		archive, e := zip.OpenReader(download)
		if e != nil {
			return e
		}
		defer archive.Close()
		found := false
		for _, f := range archive.File {
			if f.Name != name && f.Name != "geoip.dat" && f.Name != "geosite.dat" {
				continue
			}
			if f.Name != name && str(geoFileConfig(a.store.snapshot(), f.Name)["source"]) == "github" {
				continue // Preserve the explicitly selected external databases.
			}
			if f.UncompressedSize64 > 128<<20 {
				return fmt.Errorf("unpacked core too large")
			}
			reader, e := f.Open()
			if e != nil {
				return e
			}
			writer, e := os.OpenFile(filepath.Join(temp, f.Name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if e != nil {
				reader.Close()
				return e
			}
			_, e = io.Copy(writer, io.LimitReader(reader, (128<<20)+1))
			reader.Close()
			ce := writer.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
			if f.Name == name {
				found = true
			} else {
				files = append(files, f.Name)
			}
		}
		if !found {
			return fmt.Errorf("Xray binary missing in archive")
		}
	} else {
		if e = os.Rename(download, binary); e != nil {
			return e
		}
		if e = os.Chmod(binary, 0755); e != nil {
			return e
		}
	}
	if e = validateELF(binary, runtime.GOARCH); e != nil {
		return e
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, e := exec.CommandContext(checkCtx, binary, "version").CombinedOutput()
	if e != nil || !strings.Contains(string(output), requestedVersion) {
		return fmt.Errorf("downloaded core did not pass version check")
	}
	wasActive := a.core != nil && a.core.Running() && first(a.core.kind, "xray") == kind
	if wasActive {
		a.core.Stop()
	}
	undo, e := replaceCoreFiles(root, temp, files)
	if e != nil {
		if wasActive {
			if rollback := a.rollbackRuntime(a.store.snapshot(), nil); rollback != nil {
				return fmt.Errorf("core replace failed: %v; runtime rollback: %v", e, rollback)
			}
		}
		return e
	}
	if wasActive {
		d := a.store.snapshot()
		server, _ := findServer(d, str(d["selected"]), "")
		if server != nil {
			if e = a.core.ApplyServer(server, d, integer(d["proxy_port"])); e != nil {
				if rollback := undo(); rollback != nil {
					return fmt.Errorf("new core failed: %v; files rollback failed: %v", e, rollback)
				}
				if rollback := a.core.ApplyServer(server, d, integer(d["proxy_port"])); rollback != nil {
					return fmt.Errorf("new core failed: %v; old core restart failed: %v", e, rollback)
				}
				return fmt.Errorf("new core failed; previous core restored: %w", e)
			}
		}
	}
	return nil
}
