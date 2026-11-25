package netfilterTools

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var cliPolicyID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var cliInterfaceID = regexp.MustCompile(`^[a-zA-Z0-9_-]+(?:/[a-zA-Z0-9_-]+)*$`)
var cliSystemName = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,15}$`)
var cliPolicyHeader = regexp.MustCompile(`^policy, name = ([a-zA-Z0-9_-]+)(?:,|:)`)
var cliEscape = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

func parseCLIPolicyConfig(text string) (map[string]policyConfig, error) {
	config := map[string]policyConfig{}
	id := ""
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			id = ""
			if len(fields) >= 3 && fields[0] == "ip" && fields[1] == "policy" {
				if len(fields) != 3 || !cliPolicyID.MatchString(fields[2]) {
					return nil, fmt.Errorf("invalid policy header")
				}
				id = fields[2]
				if _, exists := config[id]; exists {
					return nil, fmt.Errorf("duplicate policy")
				}
				config[id] = policyConfig{}
			}
			continue
		}
		if id == "" {
			continue
		}
		no := fields[0] == "no"
		if no {
			fields = fields[1:]
		}
		if len(fields) == 0 || fields[0] != "permit" {
			continue
		}
		if len(fields) != 3 || fields[1] != "global" || !cliInterfaceID.MatchString(fields[2]) {
			return nil, fmt.Errorf("unsupported policy permit")
		}
		p := config[id]
		p.Permit = append(p.Permit, policyPermit{Interface: fields[2], Enabled: !no, No: no})
		config[id] = p
	}
	return config, nil
}

func parseCLIPolicyState(text string) (map[string]uint32, error) {
	states := map[string]uint32{}
	id := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "policy,") {
			match := cliPolicyHeader.FindStringSubmatch(line)
			if match == nil {
				return nil, fmt.Errorf("invalid runtime policy header")
			}
			id = match[1]
			continue
		}
		if id != "" && strings.HasPrefix(line, "mark:") {
			value := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "mark:")), "0x")
			mark, err := strconv.ParseUint(value, 16, 32)
			if err != nil || mark == 0 {
				return nil, fmt.Errorf("invalid runtime policy mark")
			}
			if _, exists := states[id]; exists {
				return nil, fmt.Errorf("duplicate runtime policy mark")
			}
			states[id] = uint32(mark)
		}
	}
	return states, nil
}

func (g *keeneticPolicyGuard) loadCLI() (policyScope, error) {
	read := func(command string) (string, error) {
		text, err := g.runCLI(command)
		return strings.ReplaceAll(cliEscape.ReplaceAllString(text, ""), "\r", ""), err
	}
	text, err := read("show running-config")
	if err != nil {
		return nil, err
	}
	// Reject empty/error output or an incomplete configuration, rather than permit all.
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "! $$$ Agent:") || !strings.HasSuffix(trimmed, "\n!") {
		return nil, fmt.Errorf("incomplete running configuration")
	}
	config, err := parseCLIPolicyConfig(text)
	if err != nil {
		return nil, err
	}
	text, err = read("show ip policy")
	if err != nil {
		return nil, err
	}
	states, err := parseCLIPolicyState(text)
	if err != nil {
		return nil, err
	}
	if len(states) != len(config) {
		return nil, fmt.Errorf("incomplete runtime policy list")
	}
	names := map[string]string{}
	scopes := policyScope{}
	marks := map[uint32]bool{}
	for id, p := range config {
		mark, ok := states[id]
		if !ok || marks[mark] {
			return nil, fmt.Errorf("missing or duplicate policy mark")
		}
		marks[mark] = true
		allowed := map[string]bool{}
		for _, v := range p.Permit {
			if !v.Enabled || v.No {
				continue
			}
			name := names[v.Interface]
			if name == "" {
				text, err = read("show interface " + v.Interface + " system-name")
				if err != nil {
					return nil, err
				}
				fields := strings.Fields(text)
				if len(fields) != 2 || fields[0] != "system-name:" || !cliSystemName.MatchString(fields[1]) {
					return nil, fmt.Errorf("missing interface mapping")
				}
				name = fields[1]
				names[v.Interface] = name
			}
			allowed[name] = true
		}
		scopes[mark] = allowed
	}
	return scopes, nil
}
