package netfilterTools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"optirout/utils/iptables"
)

type policyPermit struct {
	Interface string `json:"interface"`
	Enabled   bool   `json:"enabled"`
	No        bool   `json:"no"`
}
type policyConfig struct {
	Permit []policyPermit `json:"permit"`
}
type policyState struct {
	Mark string `json:"mark"`
}
type policyScope map[uint32]map[string]bool

type keeneticPolicyGuard struct {
	mu        sync.Mutex
	client    *http.Client
	baseURL   string
	required  bool
	scopes    policyScope
	links     map[*IPSetToLink]bool
	runCLI    func(string) (string, error)
	ready     bool
	checked   time.Time
	lastError string
	source    string
}

func newPolicyGuard() *keeneticPolicyGuard {
	_, err := os.Stat("/bin/ndmc")
	return &keeneticPolicyGuard{client: &http.Client{Timeout: 2 * time.Second}, baseURL: "http://127.0.0.1:79", required: err == nil, links: map[*IPSetToLink]bool{}, runCLI: func(command string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		data, e := exec.CommandContext(ctx, "/bin/ndmc", "-c", command).Output()
		if e != nil {
			return "", fmt.Errorf("ndmc read failed")
		}
		return string(data), nil
	}}
}

func (g *keeneticPolicyGuard) read(path string, out any) error {
	resp, err := g.client.Get(g.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("policy RCI status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (g *keeneticPolicyGuard) load() (policyScope, error) {
	s, err := g.loadHTTP()
	if err == nil {
		g.source = "http"
		return s, nil
	}
	if g.required && g.runCLI != nil {
		fallback, cliErr := g.loadCLI()
		if cliErr == nil {
			g.source = "cli"
			return fallback, nil
		}
		return nil, fmt.Errorf("HTTP: %v; CLI: %w", err, cliErr)
	}
	return nil, err
}

func (g *keeneticPolicyGuard) loadHTTP() (policyScope, error) {
	var config map[string]policyConfig
	var state map[string]policyState
	if err := g.read("/rci/ip/policy", &config); err != nil {
		return nil, err
	}
	if err := g.read("/rci/show/ip/policy", &state); err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, p := range config {
		for _, v := range p.Permit {
			if !seen[v.Interface] {
				seen[v.Interface] = true
				ids = append(ids, v.Interface)
			}
		}
	}
	sort.Strings(ids)
	requests := []map[string]any{}
	for _, id := range ids {
		requests = append(requests, map[string]any{"show": map[string]any{"interface": map[string]string{"name": id, "details": "yes", "system-name": "yes"}}})
	}
	names := map[string]string{}
	if len(ids) > 0 {
		body, _ := json.Marshal(requests)
		resp, err := g.client.Post(g.baseURL+"/rci/", "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("interface RCI status %d", resp.StatusCode)
		}
		var results []struct {
			Show struct {
				Interface struct {
					SystemName string `json:"system-name"`
				} `json:"interface"`
			} `json:"show"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			return nil, err
		}
		if len(results) != len(ids) {
			return nil, fmt.Errorf("incomplete interface mapping")
		}
		for i, r := range results {
			names[ids[i]] = r.Show.Interface.SystemName
		}
	}
	scopes := policyScope{}
	for id, p := range config {
		st, ok := state[id]
		if !ok {
			return nil, fmt.Errorf("policy %s has no runtime mark", id)
		}
		mark, err := strconv.ParseUint(strings.TrimPrefix(st.Mark, "0x"), 16, 32)
		if err != nil || mark == 0 {
			return nil, fmt.Errorf("invalid mark for policy %s", id)
		}
		allowed := map[string]bool{}
		for _, v := range p.Permit {
			if v.Enabled && !v.No {
				name := names[v.Interface]
				if name == "" {
					return nil, fmt.Errorf("missing system name for %s", v.Interface)
				}
				allowed[name] = true
			}
		}
		scopes[uint32(mark)] = allowed
	}
	return scopes, nil
}

// A policy mark must never be replaced by a route forbidden in that policy.
// Marks already owned by another OptiRout interface must also be preserved.
func deniedPolicyMarks(scopes policyScope, iface string, owned map[uint32]string) []uint32 {
	marks := map[uint32]bool{}
	for mark, allowed := range scopes {
		if !allowed[iface] {
			marks[mark] = true
		}
	}
	for mark, name := range owned {
		if name != iface {
			marks[mark] = true
		}
	}
	result := []uint32{}
	for mark := range marks {
		result = append(result, mark)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (nh *Helper) RefreshPolicyGuards() (refreshErr error) {
	g := nh.policyGuard
	if !g.required {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	defer func() {
		g.checked = time.Now()
		g.ready = refreshErr == nil
		g.lastError = ""
		if refreshErr != nil {
			g.lastError = refreshErr.Error()
		}
	}()
	scopes, loadErr := g.load()
	// On RCI failure skip interception, rather than bypass a device's policy.
	g.scopes = scopes
	owned := map[uint32]string{}
	for r := range g.links {
		r.locker.Lock()
		if r.enabled.Load() {
			owned[r.mark] = r.ifaceName
		}
		r.locker.Unlock()
	}
	for r := range g.links {
		r.locker.Lock()
		if r.enabled.Load() {
			for _, ipt := range []*iptables.IPTables{nh.IPTables4, nh.IPTables6} {
				if ipt != nil {
					if err := r.registerMangleRules(ipt, deniedPolicyMarks(scopes, r.ifaceName, owned), loadErr != nil); err != nil {
						r.locker.Unlock()
						return err
					}
				}
			}
		}
		r.locker.Unlock()
	}
	for _, ipt := range []*iptables.IPTables{nh.IPTables4, nh.IPTables6} {
		if ipt != nil {
			if err := ipt.Commit(); err != nil {
				return err
			}
		}
	}
	return loadErr
}

func (nh *Helper) PolicyHealth() map[string]any {
	g := nh.policyGuard
	g.mu.Lock()
	defer g.mu.Unlock()
	ready := !g.required || g.ready && !g.checked.IsZero() && time.Since(g.checked) < 30*time.Second
	return map[string]any{"ready": ready, "paused": !ready, "source": g.source, "error": g.lastError, "checked": g.checked}
}
