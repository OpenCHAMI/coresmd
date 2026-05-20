// SPDX-FileCopyrightText: Copyright 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package rule

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/openchami/coresmd/internal/iface"
)

// staticSet is a local IDSetMatcher implementation used for tests.
// It allows exercising Match.IDSet behavior without depending on CompileIDSet().
type staticSet map[string]bool

func (s staticSet) Match(id string) bool { return s[id] }

func (s staticSet) String() string { return "staticSet" }

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

func TestCreateRuleCompDict_Table(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		wantErr bool
	}{
		{"empty", " ", true},
		{"missing_colon", "hostname:a,bad", true},
		{"empty_key", ":x,hostname:a", true},
		{"unknown_key", "hostname:a,domian:oopsy", true},
		{"duplicate_key", "hostname:a,hostname:b", true},
		{"bad_quote", "hostname:'a\\'", true},
		{"ok", "name:r1,hostname:'a,b',type:Node,continue:yes,routers:192.0.2.1|192.0.2.2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createRuleCompDict(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got["hostname"] != "a,b" {
				t.Fatalf("expected hostname=%q got=%q", "a,b", got["hostname"])
			}
		})
	}
}

func TestParseRule_Table(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"missing_actions", "name:r1,type:Node", true},
		{"routers_only_ok", "routers:192.0.2.1", false},
		{"routers_bad_ip", "routers:not_an_ip", true},
		{"type_empty", "hostname:x,type:", true},
		{"type_whitespace", "hostname:x,type:   ", true},
		{"type_separators_only", "hostname:x,type:| |", true},
		{"log_invalid", "log:verbose,hostname:x", true},
		{"continue_invalid", "hostname:x,continue:maybe", true},
		{"domain_append_invalid", "hostname:x,domain_append:maybe", true},
		{"domain_append_none_combo_invalid", "hostname:x,domain_append:none|rule", true},
		{"domain_none_removed", "hostname:x,domain:none", true},
		{"subnet_invalid", "hostname:x,subnet:notacidr", true},
		{"id_and_idset_mutual_exclusion", "hostname:x,id:a,id_set:b", true},
		{"ok_minimal", "hostname:nid{04d}", false},
		{"ok_multi", "name:r1,log:debug,hostname:x,continue:yes,domain_append:global|rule,type:Node| NodeBMC ,subnet:172.16.0.0/24|172.16.1.0/24", false},
		{"ok_domain_append_rule_global", "hostname:x,domain:override.local,domain_append:rule|global", false},
		{"id_set_unimplemented", "hostname:x,id_set:x1000s[0-3]c0b0n[0-7]", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseRule(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.name != "routers_only_ok" && r.Action.Hostname == "" {
				t.Fatalf("expected non-empty hostname got=%q", r.Action.Hostname)
			}
			if tt.name == "routers_only_ok" {
				if len(r.Action.Routers) != 1 {
					t.Fatalf("expected 1 router got=%d", len(r.Action.Routers))
				}
			}
			// Ensure type trimming works for the ok_multi case.
			if tt.name == "ok_multi" {
				if r.Match.Types == nil || !r.Match.Types["Node"] || !r.Match.Types["NodeBMC"] {
					t.Fatalf("expected types to include %q and %q got=%v", "Node", "NodeBMC", r.Match.Types)
				}
				if r.Action.DomainAppend != "global|rule" {
					t.Fatalf("expected domain_append=%q got=%q", "global|rule", r.Action.DomainAppend)
				}
			}
			if tt.name == "ok_domain_append_rule_global" {
				if r.Action.DomainAppend != "rule|global" {
					t.Fatalf("expected domain_append=%q got=%q", "rule|global", r.Action.DomainAppend)
				}
			}
		})
	}
}

func TestRuleMatchIface_Combinations(t *testing.T) {
	iiNode := iface.IfaceInfo{CompID: "x1000s0c0b0n0", CompNID: 7, Type: "Node", MAC: "aa", IPList: []net.IP{net.ParseIP("172.16.0.10")}}
	iiBMC := iface.IfaceInfo{CompID: "x1000s0c0b0n0", Type: "NodeBMC", MAC: "bb", IPList: []net.IP{net.ParseIP("172.16.10.10")}}
	iiEmptyType := iface.IfaceInfo{CompID: "x1000s0c0b0n0", CompNID: 7, Type: "", MAC: "cc", IPList: []net.IP{net.ParseIP("172.16.0.10")}}

	mset := staticSet{"x1000s0c0b0n0": true, "x1000s0c0b0n1": true}

	tests := []struct {
		name      string
		rule      Rule
		ii        iface.IfaceInfo
		wantMatch bool
	}{
		{"match_all_when_no_match_fields", Rule{Action: Action{Hostname: "x"}}, iiNode, true},
		{"empty_type_map_is_wildcard_matches_empty_type", Rule{Match: Match{Types: map[string]bool{}}, Action: Action{Hostname: "x"}}, iiEmptyType, true},
		{"type_match", Rule{Match: Match{Types: map[string]bool{"Node": true}}, Action: Action{Hostname: "x"}}, iiNode, true},
		{"type_mismatch", Rule{Match: Match{Types: map[string]bool{"Node": true}}, Action: Action{Hostname: "x"}}, iiBMC, false},
		{"subnet_match", Rule{Match: Match{Subnets: []*net.IPNet{mustCIDR(t, "172.16.0.0/24")}}, Action: Action{Hostname: "x"}}, iiNode, true},
		{"id_match_trim", Rule{Match: Match{ID: "  x1000s0c0b0n0  "}, Action: Action{Hostname: "x"}}, iiNode, true},
		{"idset_match", Rule{Match: Match{IDSet: mset}, Action: Action{Hostname: "x"}}, iiNode, true},
		{"compound_all_required", Rule{Match: Match{Types: map[string]bool{"Node": true}, Subnets: []*net.IPNet{mustCIDR(t, "172.16.0.0/24")}, IDSet: mset}, Action: Action{Hostname: "x"}}, iiNode, true},
		{"compound_missing_one", Rule{Match: Match{Types: map[string]bool{"Node": true}, Subnets: []*net.IPNet{mustCIDR(t, "172.16.99.0/24")}, IDSet: mset}, Action: Action{Hostname: "x"}}, iiNode, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := tt.rule.MatchIface(tt.ii)
			if m != tt.wantMatch {
				t.Fatalf("expected match=%v got=%v", tt.wantMatch, m)
			}
		})
	}
}

func TestEvaluate4_HostnameRoutersAndDefault(t *testing.T) {
	ii := iface.IfaceInfo{CompID: "x1000s0c0b0n0", CompNID: 7, Type: "Node", MAC: "aa", IPList: []net.IP{net.ParseIP("172.16.0.10")}}

	// Matching rules set hostname and routers; default should NOT override.
	resp, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("unexpected error creating dhcpv4 message: %v", err)
	}
	rules := []Rule{{
		Name:   "node",
		Match:  Match{Types: map[string]bool{"Node": true}},
		Action: Action{Hostname: "nid{04d}", Domain: "override.local", Routers: []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}},
	}}
	Evaluate4(nil, ii, "cluster.local", "none", resp, rules)

	if got := string(bytes.Trim(resp.Options.Get(dhcpv4.OptionHostName), "\x00")); got != "nid0007.override.local" {
		t.Fatalf("expected=%q got=%q", "nid0007.override.local", got)
	}
	if got := resp.Options.Get(dhcpv4.OptionRouter); len(got) != 8 {
		t.Fatalf("expected %d bytes of router option got=%d", 8, len(got))
	}

	// Routers-only rule is allowed; hostname falls back to DefaultPattern.
	resp2, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("unexpected error creating dhcpv4 message: %v", err)
	}
	rules2 := []Rule{{Name: "rtrs", Action: Action{Routers: []net.IP{net.ParseIP("192.0.2.1")}}}}
	Evaluate4(nil, ii, "cluster.local", "none", resp2, rules2)
	if got := string(bytes.Trim(resp2.Options.Get(dhcpv4.OptionHostName), "\x00")); got != "unknown-0007.cluster.local" {
		t.Fatalf("expected=%q got=%q", "unknown-0007.cluster.local", got)
	}

	// No rules match => default hostname applied.
	resp3, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("unexpected error creating dhcpv4 message: %v", err)
	}
	rules3 := []Rule{{Name: "nope", Match: Match{Types: map[string]bool{"NodeBMC": true}}, Action: Action{Hostname: "bmc{04d}"}}}
	Evaluate4(nil, ii, "cluster.local", "none", resp3, rules3)
	if got := string(bytes.Trim(resp3.Options.Get(dhcpv4.OptionHostName), "\x00")); got != "unknown-0007.cluster.local" {
		t.Fatalf("expected=%q got=%q", "unknown-0007.cluster.local", got)
	}

	// Subnet match only checks the first IP in the list.
	ii2 := iface.IfaceInfo{CompID: "x1000s0c0b0n0", CompNID: 7, Type: "Node", MAC: "aa", IPList: []net.IP{net.ParseIP("172.16.99.10"), net.ParseIP("172.16.0.10")}}
	resp4, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("unexpected error creating dhcpv4 message: %v", err)
	}
	rules4 := []Rule{{Name: "subnet", Match: Match{Subnets: []*net.IPNet{mustCIDR(t, "172.16.0.0/24")}}, Action: Action{Hostname: "nid{04d}"}}}
	Evaluate4(nil, ii2, "cluster.local", "none", resp4, rules4)
	if got := string(bytes.Trim(resp4.Options.Get(dhcpv4.OptionHostName), "\x00")); got != "unknown-0007.cluster.local" {
		t.Fatalf("expected=%q got=%q", "unknown-0007.cluster.local", got)
	}
}

func TestEvaluate6_HostnameAndDefault(t *testing.T) {
	ii := iface.IfaceInfo{CompID: "x1000s0c0b0n0", CompNID: 7, Type: "Node", MAC: "aa", IPList: []net.IP{net.ParseIP("172.16.0.10")}}

	resp, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatalf("unexpected error creating dhcpv6 message: %v", err)
	}
	rules := []Rule{{Name: "node", Match: Match{Types: map[string]bool{"Node": true}}, Action: Action{Hostname: "nid{04d}", Domain: "override.local"}}}
	Evaluate6(nil, ii, "cluster.local", "none", resp, rules)

	opt := resp.GetOneOption(dhcpv6.OptionFQDN)
	if opt == nil {
		t.Fatalf("expected FQDN option to be set got=nil")
	}
	fqdn, ok := opt.(*dhcpv6.OptFQDN)
	if !ok {
		t.Fatalf("expected OptFQDN got=%T", opt)
	}
	got := strings.Join(fqdn.DomainName.Labels, ".")
	if got != "nid0007.override.local" {
		t.Fatalf("expected=%q got=%q", "nid0007.override.local", got)
	}

	// No match => default hostname applied.
	resp2, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatalf("unexpected error creating dhcpv6 message: %v", err)
	}
	rules2 := []Rule{{Name: "nope", Match: Match{Types: map[string]bool{"NodeBMC": true}}, Action: Action{Hostname: "bmc{04d}"}}}
	Evaluate6(nil, ii, "cluster.local", "none", resp2, rules2)
	opt2 := resp2.GetOneOption(dhcpv6.OptionFQDN)
	if opt2 == nil {
		t.Fatalf("expected FQDN option to be set got=nil")
	}
	fqdn2 := opt2.(*dhcpv6.OptFQDN)
	got2 := strings.Join(fqdn2.DomainName.Labels, ".")
	if got2 != "unknown-0007.cluster.local" {
		t.Fatalf("expected=%q got=%q", "unknown-0007.cluster.local", got2)
	}
}

// TestParseRule_Ignore tests parsing of the ignore action
func TestParseRule_Ignore(t *testing.T) {
	tests := []struct {
		name       string
		rule       string
		wantIgnore bool
		wantErr    bool
	}{
		{"ignore_true", "ignore:true", true, false},
		{"ignore_false", "ignore:false,hostname:test", false, false},
		{"ignore_yes", "ignore:yes", true, false},
		{"ignore_no", "ignore:no,hostname:test", false, false},
		{"ignore_1", "ignore:1", true, false},
		{"ignore_0", "ignore:0,hostname:test", false, false},
		{"ignore_invalid", "ignore:maybe,hostname:test", false, true},
		{"ignore_with_subnet", "subnet:172.16.0.0/24,ignore:true", true, false},
		{"ignore_with_type", "type:RouterBMC,ignore:true", true, false},
		{"ignore_with_hostname", "type:Node,hostname:compute-{04d},ignore:true", true, false},
		{"ignore_alone_valid", "ignore:true", true, false},
		{"ignore_false_no_other_action", "ignore:false", false, true}, // Should fail validation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRule(%q): err=%v wantErr=%v", tt.rule, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if r.Action.Ignore != tt.wantIgnore {
				t.Fatalf("ParseRule(%q): got Ignore=%v want=%v", tt.rule, r.Action.Ignore, tt.wantIgnore)
			}
		})
	}
}

// TestEvaluate4_Ignore tests that DHCPv4 requests are dropped when ignore=true
func TestEvaluate4_Ignore(t *testing.T) {
	// Test component info
	ii := iface.IfaceInfo{
		CompID:  "x3000c0s0b0n0",
		CompNID: 5,
		Type:    "Node",
		MAC:     "de:ad:be:ef:00:01",
		IPList:  []net.IP{net.ParseIP("172.16.0.10")},
	}

	// Create a DHCPv4 response
	resp, err := dhcpv4.NewReplyFromRequest(&dhcpv4.DHCPv4{})
	if err != nil {
		t.Fatalf("failed to create DHCPv4 response: %v", err)
	}

	t.Run("ignore_true_drops_request", func(t *testing.T) {
		rules := []Rule{{
			Name:   "drop_all",
			Match:  Match{Types: map[string]bool{"Node": true}},
			Action: Action{Ignore: true},
		}}

		shouldRespond := Evaluate4(nil, ii, "test.local", "info", resp, rules)
		if shouldRespond {
			t.Fatalf("expected shouldRespond=false got=true")
		}

		// Verify no hostname was set (request dropped before actions)
		if len(resp.Options.Get(dhcpv4.OptionHostName)) > 0 {
			t.Fatalf("expected no hostname to be set when ignored")
		}
	})

	t.Run("ignore_false_allows_request", func(t *testing.T) {
		resp2, _ := dhcpv4.NewReplyFromRequest(&dhcpv4.DHCPv4{})
		rules := []Rule{{
			Name:   "allow_with_hostname",
			Match:  Match{Types: map[string]bool{"Node": true}},
			Action: Action{Ignore: false, Hostname: "test-{04d}"},
		}}

		shouldRespond := Evaluate4(nil, ii, "test.local", "info", resp2, rules)
		if !shouldRespond {
			t.Fatalf("expected shouldRespond=true got=false")
		}

		// Verify hostname was set
		hostname := string(resp2.Options.Get(dhcpv4.OptionHostName))
		if !strings.HasPrefix(hostname, "test-") {
			t.Fatalf("expected hostname to be set, got=%q", hostname)
		}
	})

	t.Run("ignore_takes_precedence", func(t *testing.T) {
		resp3, _ := dhcpv4.NewReplyFromRequest(&dhcpv4.DHCPv4{})
		rules := []Rule{{
			Name:   "ignore_with_other_actions",
			Match:  Match{Types: map[string]bool{"Node": true}},
			Action: Action{Ignore: true, Hostname: "should-not-set"},
		}}

		shouldRespond := Evaluate4(nil, ii, "test.local", "info", resp3, rules)
		if shouldRespond {
			t.Fatalf("expected shouldRespond=false (ignore takes precedence)")
		}

		// Verify hostname was NOT set
		if len(resp3.Options.Get(dhcpv4.OptionHostName)) > 0 {
			t.Fatalf("expected no hostname when ignored, got=%q", resp3.Options.Get(dhcpv4.OptionHostName))
		}
	})

	t.Run("no_match_continues_to_default", func(t *testing.T) {
		resp4, _ := dhcpv4.NewReplyFromRequest(&dhcpv4.DHCPv4{})
		rules := []Rule{{
			Name:   "wont_match",
			Match:  Match{Types: map[string]bool{"RouterBMC": true}},
			Action: Action{Ignore: true},
		}}

		shouldRespond := Evaluate4(nil, ii, "test.local", "info", resp4, rules)
		if !shouldRespond {
			t.Fatalf("expected shouldRespond=true when no rule matches")
		}

		// Verify default hostname was set
		hostname := string(resp4.Options.Get(dhcpv4.OptionHostName))
		if !strings.HasPrefix(hostname, "unknown-") {
			t.Fatalf("expected default hostname pattern, got=%q", hostname)
		}
	})
}

// TestEvaluate6_Ignore tests that DHCPv6 requests are dropped when ignore=true
func TestEvaluate6_Ignore(t *testing.T) {
	// Test component info with IPv6
	ii := iface.IfaceInfo{
		CompID:  "x3000c0s0b0n0",
		CompNID: 5,
		Type:    "Node",
		MAC:     "de:ad:be:ef:00:01",
		IPList:  []net.IP{net.ParseIP("2001:db8::1")},
	}

	t.Run("ignore_true_drops_request", func(t *testing.T) {
		resp, err := dhcpv6.NewMessage()
		if err != nil {
			t.Fatalf("failed to create DHCPv6 response: %v", err)
		}

		rules := []Rule{{
			Name:   "drop_all_v6",
			Match:  Match{Types: map[string]bool{"Node": true}},
			Action: Action{Ignore: true},
		}}

		shouldRespond := Evaluate6(nil, ii, "test.local", "info", resp, rules)
		if shouldRespond {
			t.Fatalf("expected shouldRespond=false got=true")
		}

		// Verify no FQDN was set
		if opt := resp.GetOneOption(dhcpv6.OptionFQDN); opt != nil {
			t.Fatalf("expected no FQDN when ignored, got=%v", opt)
		}
	})

	t.Run("ignore_false_allows_request", func(t *testing.T) {
		resp, _ := dhcpv6.NewMessage()
		rules := []Rule{{
			Name:   "allow_with_hostname_v6",
			Match:  Match{Types: map[string]bool{"Node": true}},
			Action: Action{Ignore: false, Hostname: "test-{04d}"},
		}}

		shouldRespond := Evaluate6(nil, ii, "test.local", "info", resp, rules)
		if !shouldRespond {
			t.Fatalf("expected shouldRespond=true got=false")
		}

		// Verify FQDN was set
		opt := resp.GetOneOption(dhcpv6.OptionFQDN)
		if opt == nil {
			t.Fatalf("expected FQDN option to be set")
		}
	})
}
