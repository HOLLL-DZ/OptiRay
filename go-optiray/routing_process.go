package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type RoutingProcess struct {
	mu                      sync.Mutex
	bin, dir, productionPID string
	cmd                     *exec.Cmd
	done                    chan error
	lastExit                error
}

func routingEnvironment(dir string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "OPTIRAY_ROUTING_") {
			env = append(env, entry)
		}
	}
	return append(env, "OPTIRAY_ROUTING_CONFIG_DIR="+filepath.Join(dir, "routing"), "OPTIRAY_ROUTING_STATE_DIR="+filepath.Join(dir, "routing"), "OPTIRAY_ROUTING_PID="+filepath.Join(dir, "routing", "engine.pid"), "OPTIRAY_ROUTING_SOCKET="+filepath.Join(dir, "routing", "engine.sock"))
}
func (p *RoutingProcess) runningLocked() bool {
	if p.cmd == nil {
		return false
	}
	select {
	case e := <-p.done:
		p.lastExit = e
		p.cmd = nil
		return false
	default:
		return true
	}
}
func (p *RoutingProcess) Status() Object {
	p.mu.Lock()
	defer p.mu.Unlock()
	running := p.runningLocked()
	var pid any
	if running {
		pid = p.cmd.Process.Pid
	}
	return Object{"running": running, "pid": pid, "bin_name": "optiroutd", "bin_path": p.bin, "init_path": nil, "config_path": filepath.Join(p.dir, "routing", "optirout.json"), "config_format": "json", "supported": true}
}
func (p *RoutingProcess) stopLocked() error {
	if !p.runningLocked() {
		e := p.lastExit
		p.lastExit = nil
		return e
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case e := <-p.done:
		p.cmd = nil
		if e != nil {
			return fmt.Errorf("OptiRout shutdown failed; inspect routing/engine.log: %w", e)
		}
		return nil
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
		p.cmd = nil
		return fmt.Errorf("OptiRout required forced shutdown; router rules need checking")
	}
}
func (p *RoutingProcess) Stop() error { p.mu.Lock(); defer p.mu.Unlock(); return p.stopLocked() }
func (p *RoutingProcess) Restart(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if p.productionPID != "" {
		if _, e := os.Stat(p.productionPID); e == nil {
			return fmt.Errorf("production OptiRout PID file exists; refusing parallel routing engines")
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	if e := p.stopLocked(); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Join(p.dir, "routing"), 0700); e != nil {
		return e
	}
	socket := filepath.Join(p.dir, "routing", "engine.sock")
	if conn, e := net.DialTimeout("unix", socket, 100*time.Millisecond); e == nil {
		conn.Close()
		return fmt.Errorf("another process owns the routing socket")
	}
	logfile, e := os.OpenFile(filepath.Join(p.dir, "routing", "engine.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	cmd := exec.Command(p.bin)
	cmd.Dir = p.dir
	cmd.Env = routingEnvironment(p.dir)
	cmd.Stdout = logfile
	cmd.Stderr = logfile
	if e = cmd.Start(); e != nil {
		logfile.Close()
		return e
	}
	p.cmd = cmd
	p.lastExit = nil
	p.done = make(chan error, 1)
	done := p.done
	go func() { done <- cmd.Wait(); logfile.Close() }()
	deadline := time.NewTimer(8 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = p.stopLocked()
			return ctx.Err()
		case <-done:
			p.cmd = nil
			return fmt.Errorf("OptiRout exited before readiness; see routing/engine.log")
		case <-deadline.C:
			_ = p.stopLocked()
			return fmt.Errorf("OptiRout socket did not become ready")
		case <-ticker.C:
			if conn, e := net.DialTimeout("unix", socket, 100*time.Millisecond); e == nil {
				conn.Close()
				return nil
			}
		}
	}
}
