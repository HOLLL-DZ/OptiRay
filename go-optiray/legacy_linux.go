//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type legacyProcess struct {
	pid            int
	role, exe, cwd string
	args           []string
}

func readLegacyProcess(pid int) legacyProcess {
	base := filepath.Join("/proc", strconv.Itoa(pid))
	exe, _ := os.Readlink(base + "/exe")
	cwd, _ := os.Readlink(base + "/cwd")
	raw, _ := os.ReadFile(base + "/cmdline")
	args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	// /opt may itself resolve to mounted storage on Keenetic.
	for _, path := range []string{"/opt/bin/xray", "/opt/bin/hysteria", "/opt/etc/optiray/bin/optiroutd"} {
		if actual, e := filepath.EvalSymlinks(path); e == nil && actual == exe {
			exe = path
		}
	}
	if actual, e := filepath.EvalSymlinks("/opt/etc/optiray"); e == nil && cwd == actual {
		cwd = "/opt/etc/optiray"
	}
	enginePID, _ := os.ReadFile("/opt/var/run/optirout.pid")
	return legacyProcess{pid: pid, exe: exe, cwd: cwd, args: args, role: legacyProcessRole(exe, cwd, args, strings.TrimSpace(string(enginePID)) == strconv.Itoa(pid))}
}
func legacyProcesses() []legacyProcess {
	entries, _ := os.ReadDir("/proc")
	out := []legacyProcess{}
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e == nil {
			p := readLegacyProcess(pid)
			if p.role != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
func legacyRunning() bool { return len(legacyProcesses()) > 0 }
func stopLegacyInstallation() error {
	for _, role := range []string{"watchdog", "manager", "core", "routing"} {
		for _, p := range legacyProcesses() {
			if p.role != role {
				continue
			}
			current := readLegacyProcess(p.pid)
			if current.role != role || current.exe != p.exe {
				continue
			}
			process, e := os.FindProcess(p.pid)
			if e != nil {
				return e
			}
			if e = process.Signal(syscall.SIGTERM); e != nil && !os.IsNotExist(e) {
				return e
			}
			deadline := time.Now().Add(25 * time.Second)
			for readLegacyProcess(p.pid).role == role {
				if time.Now().After(deadline) {
					return fmt.Errorf("legacy %s PID %d did not stop; refusing forced routing cleanup", role, p.pid)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	if legacyRunning() {
		return fmt.Errorf("legacy OptiRay is still running")
	}
	for _, path := range []string{"/var/run/optiray.pid", "/opt/var/run/optirout.pid"} {
		raw, e := os.ReadFile(path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		pid, e := strconv.Atoi(strings.TrimSpace(string(raw)))
		if e != nil {
			return fmt.Errorf("invalid legacy PID file: %s", path)
		}
		if _, e = os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); e == nil {
			return fmt.Errorf("legacy PID file now belongs to another process")
		}
		if e = os.Remove(path); e != nil {
			return e
		}
	}
	return nil
}
func startLegacyInstallation(dir string) error {
	if legacyRunning() {
		return fmt.Errorf("refusing duplicate legacy startup")
	}
	routing := "/opt/etc/optiray/routing.sh"
	if _, e := os.Stat(routing); e == nil {
		if e = exec.Command(routing, "start").Run(); e != nil {
			return e
		}
	}
	signal.Ignore(syscall.SIGHUP)
	launch := func(bin, script, logPath string) (int, error) {
		log, e := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return 0, e
		}
		defer log.Close()
		cmd := exec.Command(bin, script)
		cmd.Dir = "/opt/etc/optiray"
		if auth, e := readObject(filepath.Join(dir, "legacy-auth.json")); e == nil && boolean(auth["enabled"]) {
			cmd.Env = append(os.Environ(), "OPTIRAY_USER="+str(auth["user"]), "OPTIRAY_PASS="+str(auth["pass"]))
		}
		cmd.Stdout = log
		cmd.Stderr = log
		if e = cmd.Start(); e != nil {
			return 0, e
		}
		return cmd.Process.Pid, nil
	}
	python, e := exec.LookPath("python3")
	if e != nil {
		return e
	}
	pid, e := launch(python, "/opt/etc/optiray/app.py", "/opt/var/log/optiray_startup.log")
	if e != nil {
		return e
	}
	if e = atomicBytes("/var/run/optiray.pid", []byte(strconv.Itoa(pid)+"\n")); e != nil {
		return e
	}
	if _, e = os.Stat("/opt/etc/optiray/watchdog.sh"); e == nil {
		_, e = launch("/bin/sh", "/opt/etc/optiray/watchdog.sh", "/opt/var/log/optiray_watchdog.log")
	}
	if os.IsNotExist(e) {
		return nil
	}
	return e
}

func captureLegacyAuthentication(dir string) error {
	if _, e := os.Stat("/opt/etc/optiray/auth.json"); e == nil {
		return nil
	}
	for _, p := range legacyProcesses() {
		if p.role == "manager" {
			raw, e := os.ReadFile(filepath.Join("/proc", strconv.Itoa(p.pid), "environ"))
			if e != nil {
				return e
			}
			if auth := authenticationFromEnvironment(raw); auth != nil {
				return writeObject(filepath.Join(dir, "legacy-auth.json"), auth)
			}
			return nil
		}
	}
	raw, e := os.ReadFile("/opt/etc/init.d/S99optiray")
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	auth, e := legacyAuthenticationFromService(string(raw))
	if e != nil {
		return e
	}
	if auth != nil {
		return writeObject(filepath.Join(dir, "legacy-auth.json"), auth)
	}
	return nil
}
