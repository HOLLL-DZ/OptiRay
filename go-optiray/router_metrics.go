package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type routerCPUSample struct{ total, idle uint64 }
type routerMetricsState struct {
	mu       sync.Mutex
	previous routerCPUSample
	at       time.Time
	cpu      *float64
}

func parseRouterCPU(data string) (routerCPUSample, error) {
	fields := strings.Fields(strings.SplitN(data, "\n", 2)[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return routerCPUSample{}, fmt.Errorf("CPU unavailable")
	}
	var s routerCPUSample
	for i, v := range fields[1:] {
		if i >= 8 {
			break
		}
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return s, err
		}
		s.total += n
		if i == 3 || i == 4 {
			s.idle += n
		}
	}
	return s, nil
}
func parseRouterMemory(data string) (uint64, uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			n, e := strconv.ParseUint(f[1], 10, 64)
			if e == nil {
				values[strings.TrimSuffix(f[0], ":")] = n
			}
		}
	}
	total := values["MemTotal"]
	if total == 0 {
		return 0, 0, fmt.Errorf("Memory unavailable")
	}
	available, ok := values["MemAvailable"]
	if !ok {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if available > total {
		available = total
	}
	return total, total - available, nil
}
func (a *App) routerMetrics(w http.ResponseWriter, r *http.Request) {
	stat, e := os.ReadFile("/proc/stat")
	if e != nil {
		jsonReply(w, 503, Object{"ok": false})
		return
	}
	sample, e := parseRouterCPU(string(stat))
	if e != nil {
		jsonReply(w, 503, Object{"ok": false})
		return
	}
	a.metrics.mu.Lock()
	now := time.Now()
	if a.metrics.at.IsZero() {
		a.metrics.previous = sample
		a.metrics.at = now
	} else if now.Sub(a.metrics.at) >= time.Second {
		old := a.metrics.previous
		if sample.total > old.total && sample.idle >= old.idle {
			total := sample.total - old.total
			idle := sample.idle - old.idle
			if idle <= total {
				v := 100 * float64(total-idle) / float64(total)
				a.metrics.cpu = &v
			}
		}
		a.metrics.previous = sample
		a.metrics.at = now
	}
	var cpu any
	if a.metrics.cpu != nil {
		cpu = *a.metrics.cpu
	}
	a.metrics.mu.Unlock()
	mem, e := os.ReadFile("/proc/meminfo")
	if e != nil {
		jsonReply(w, 503, Object{"ok": false})
		return
	}
	total, used, e := parseRouterMemory(string(mem))
	if e != nil {
		jsonReply(w, 503, Object{"ok": false})
		return
	}
	model := "Роутер"
	for _, path := range []string{"/proc/device-tree/model", "/sys/firmware/devicetree/base/model"} {
		if b, e := os.ReadFile(path); e == nil && len(b) > 0 {
			model = strings.TrimSpace(strings.Trim(string(b), "\x00"))
			break
		}
	}
	if system := readKeeneticMetrics(); system != nil {
		system["uptime_seconds"] = routerUptimeSeconds()
		system["model"] = model
		jsonReply(w, 200, system)
		return
	}
	jsonReply(w, 200, Object{"ok": true, "model": model, "uptime_seconds": routerUptimeSeconds(), "cpu_percent": cpu, "memory_percent": 100 * float64(used) / float64(total), "memory_used_mb": used / 1024, "memory_total_mb": total / 1024})
}

var keeneticMetricsCache struct {
	sync.Mutex
	at   time.Time
	data Object
}

func readKeeneticMetrics() Object {
	keeneticMetricsCache.Lock()
	defer keeneticMetricsCache.Unlock()
	if time.Since(keeneticMetricsCache.at) < 2*time.Second {
		return copyMetricsObject(keeneticMetricsCache.data)
	}
	keeneticMetricsCache.at = time.Now()
	keeneticMetricsCache.data = nil
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	path, e := exec.LookPath("ndmc")
	if e != nil {
		path = "/bin/ndmc"
	}
	output, e := exec.CommandContext(ctx, path, "-c", "show system").Output()
	if e != nil {
		return nil
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	memory := strings.Split(values["memory"], "/")
	if len(memory) != 2 {
		return nil
	}
	used, e1 := strconv.ParseUint(memory[0], 10, 64)
	total, e2 := strconv.ParseUint(memory[1], 10, 64)
	cpu, e3 := strconv.ParseFloat(values["cpuload"], 64)
	if e1 != nil || e2 != nil || e3 != nil || total == 0 || used > total || cpu < 0 || cpu > 100 {
		return nil
	}
	data := Object{"ok": true, "cpu_percent": cpu, "memory_percent": 100 * float64(used) / float64(total), "memory_used_mb": used / 1024, "memory_total_mb": total / 1024, "source": "keenetic"}
	keeneticMetricsCache.data = data
	return copyMetricsObject(data)
}

func copyMetricsObject(data Object) Object {
	if data == nil {
		return nil
	}
	out := Object{}
	for k, v := range data {
		out[k] = v
	}
	return out
}

func routerUptimeSeconds() uint64 {
	data, e := os.ReadFile("/proc/uptime")
	if e != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	n, e := strconv.ParseFloat(fields[0], 64)
	if e != nil || n < 0 {
		return 0
	}
	return uint64(n)
}

func (a *App) routerReboot(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-OptiRay-Action") != "reboot" {
		jsonReply(w, 403, Object{"ok": false, "error": "Подтверждение отсутствует"})
		return
	}
	path, e := exec.LookPath("ndmc")
	if e != nil {
		path = "/bin/ndmc"
	}
	if _, e = os.Stat(path); e != nil {
		jsonReply(w, 503, Object{"ok": false, "error": "Команда Keenetic недоступна"})
		return
	}
	jsonReply(w, 200, Object{"ok": true})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, path, "-c", "system reboot").Run()
	}()
}
