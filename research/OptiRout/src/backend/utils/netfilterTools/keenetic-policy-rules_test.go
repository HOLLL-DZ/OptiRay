//go:build testing

package netfilterTools

import (
	"optirout/utils/iptables"
	"testing"
)

func TestPolicyGuardPrecedesRouting(t *testing.T) {
	for _, proto := range []iptables.Protocol{iptables.ProtocolIPv4, iptables.ProtocolIPv6} {
		fake := iptables.NewFakeIPTables(proto)
		ipt := iptables.NewIPTables(fake)
		r := &IPSetToLink{chainName: "OR_test", ifaceName: "t2s0", mark: 1298229097, ipset: &IPSet{ipsetName: "or_test"}}
		if err := r.registerMangleRules(ipt, []uint32{0xffffaab}, false); err != nil {
			t.Fatal(err)
		}
		if err := ipt.Commit(); err != nil {
			t.Fatal(err)
		}
		rules := fake.GetRules("mangle", "OR_test")
		if len(rules) != 4 || rules[1][2] != "--mark" || rules[1][3] != "268434091" || rules[1][5] != "RETURN" {
			t.Fatalf("policy must return before MARK: %#v", rules)
		}
		r.registerMangleRules(ipt, nil, true)
		ipt.Commit()
		rules = fake.GetRules("mangle", "OR_test")
		if rules[1][1] != "RETURN" {
			t.Fatal("unavailable policy data must pause interception")
		}
		r.ifaceName = Blackhole
		r.registerMangleRules(ipt, []uint32{0xffffaab}, true)
		ipt.Commit()
		if len(fake.GetRules("mangle", "OR_test")) != 3 {
			t.Fatal("blocking must remain active")
		}
	}
}
