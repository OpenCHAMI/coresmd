// SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package coresmd

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/openchami/coresmd/internal/cache"
	"github.com/openchami/coresmd/internal/rule"
	"github.com/openchami/coresmd/internal/tftp"
)

func TestConfigString_IncludesKeyFields(t *testing.T) {
	svc, _ := url.Parse("https://svc.example.test")
	ipxe, _ := url.Parse("https://ipxe.example.test")
	cacheDur := 10 * time.Second
	leaseDur := 5 * time.Minute

	cfg := Config{
		svcBaseURI:  svc,
		ipxeBaseURI: ipxe,
		caCert:      "/etc/ssl/ca.pem",
		cacheValid:  &cacheDur,
		leaseTime:   &leaseDur,
		singlePort:  true,
		tftpDir:     "/tftp",
		tftpPort:    1069,
		domain:      "example.test",
		ruleLog:     "debug",
	}

	s := cfg.String()
	wantSubstrings := []string{
		"svc_base_uri=" + svc.String(),
		"ipxe_base_uri=" + ipxe.String(),
		"ca_cert=/etc/ssl/ca.pem",
		"cache_valid=" + cacheDur.String(),
		"lease_time=" + leaseDur.String(),
		"single_port=true",
		"tftp_dir=/tftp",
		"tftp_port=1069",
		"domain=example.test",
		"rule_log=debug",
	}
	for _, sub := range wantSubstrings {
		if !strings.Contains(s, sub) {
			t.Fatalf("Config.String() missing %q in %q", sub, s)
		}
	}
}

func TestParseConfig_Rules(t *testing.T) {
	cacheDur := 15 * time.Second
	leaseDur := 30 * time.Minute

	args := []string{
		"svc_base_uri=https://svc.example.test",
		"ipxe_base_uri=https://ipxe.example.test",
		"ca_cert=/etc/pki/ca.pem",
		"cache_valid=" + cacheDur.String(),
		"lease_time=" + leaseDur.String(),
		"single_port=true",
		"tftp_dir=/tftp",
		"tftp_port=1069",
		"domain=cluster.local",
		"rule_log=info",
		"rule=name:special,type:Node,id:x1000s0c0b0n0,hostname:login-{id},domain:mgmt.local",
	}

	cfg, errs := parseConfig(args...)
	if len(errs) != 0 {
		t.Fatalf("parseConfig() unexpected errors: %v", errs)
	}
	if cfg.ruleLog != "info" {
		t.Fatalf("ruleLog=%q", cfg.ruleLog)
	}
	if len(cfg.rules) != 1 {
		t.Fatalf("rules len=%d, want 1", len(cfg.rules))
	}
	found := false
	for _, r := range cfg.rules {
		if r.Name == "special" {
			found = true
			// Rule-level log is omitted; it should inherit from global rule_log.
			if r.Log != "" {
				t.Fatalf("expected rule.Log to be empty before validate, got %q", r.Log)
			}
			if r.Match.ID != "x1000s0c0b0n0" {
				t.Fatalf("special rule id=%q", r.Match.ID)
			}
			if r.Match.Types == nil || !r.Match.Types["Node"] {
				t.Fatalf("special rule type match missing")
			}
			if r.Action.Hostname != "login-{id}" {
				t.Fatalf("special rule hostname=%q", r.Action.Hostname)
			}
			if r.Action.Domain != "mgmt.local" {
				t.Fatalf("special rule domain=%q", r.Action.Domain)
			}
		}
	}
	if !found {
		t.Fatalf("did not find explicit rule named 'special'")
	}
}

func TestConfigValidate_RuleLogInheritance(t *testing.T) {
	svc, _ := url.Parse("https://svc.example.test")
	ipxe, _ := url.Parse("https://ipxe.example.test")

	// Create a config with a rule that omits Log; validate should set the
	// effective per-rule log to the global rule_log value.
	cfg := Config{svcBaseURI: svc, ipxeBaseURI: ipxe, ruleLog: "debug"}
	cfg.rules = []rule.Rule{{Name: "r1", Log: "", Action: rule.Action{Hostname: "nid{04d}"}}}

	_, errs := cfg.validate()
	if len(errs) != 0 {
		t.Fatalf("validate() errs=%v", errs)
	}
	if cfg.rules[0].Log != "debug" {
		t.Fatalf("expected inherited rule log %q got %q", "debug", cfg.rules[0].Log)
	}

	// Explicit rule log must override global.
	cfg = Config{svcBaseURI: svc, ipxeBaseURI: ipxe, ruleLog: "debug"}
	cfg.rules = []rule.Rule{{Name: "r1", Log: "none", Action: rule.Action{Hostname: "nid{04d}"}}}
	_, errs = cfg.validate()
	if len(errs) != 0 {
		t.Fatalf("validate() errs=%v", errs)
	}
	if cfg.rules[0].Log != "none" {
		t.Fatalf("expected explicit rule log %q got %q", "none", cfg.rules[0].Log)
	}
}

func TestConfigValidate_DefaultsApplied(t *testing.T) {
	svc, _ := url.Parse("https://svc.example.test")
	ipxe, _ := url.Parse("https://ipxe.example.test")

	cfg := Config{svcBaseURI: svc, ipxeBaseURI: ipxe}
	warns, errs := cfg.validate()
	if len(errs) != 0 {
		t.Fatalf("validate() errs=%v", errs)
	}
	if len(warns) == 0 {
		t.Fatalf("validate() expected warnings, got none")
	}
	if cfg.cacheValid == nil || cfg.cacheValid.String() != cache.DefaultCacheValid {
		t.Fatalf("cacheValid=%v want %s", cfg.cacheValid, cache.DefaultCacheValid)
	}
	if cfg.leaseTime == nil || cfg.leaseTime.String() != defaultLeaseTime {
		t.Fatalf("leaseTime=%v want %s", cfg.leaseTime, defaultLeaseTime)
	}
	if cfg.tftpPort != tftp.DefaultTFTPPort {
		t.Fatalf("tftpPort=%d want %d", cfg.tftpPort, tftp.DefaultTFTPPort)
	}
	if cfg.tftpDir != tftp.DefaultTFTPDirectory {
		t.Fatalf("tftpDir=%q want %q", cfg.tftpDir, tftp.DefaultTFTPDirectory)
	}
	if cfg.ruleLog != "info" {
		t.Fatalf("ruleLog=%q want %q", cfg.ruleLog, "info")
	}
}

func TestParseConfig_SubnetAutoBuiltFromRules(t *testing.T) {
	base := []string{
		"svc_base_uri=https://svc.example.test",
		"ipxe_base_uri=https://ipxe.example.test",
	}

	// Single rule with subnet match auto-builds SubnetContext
	args := append(base,
		"rule=subnet:10.40.1.0/24,type:Node,hostname:compute-{04d},routers:10.40.1.1,cidr:24",
	)
	cfg, errs := parseConfig(args...)
	if len(errs) != 0 {
		t.Fatalf("single rule with subnet: unexpected errors: %v", errs)
	}
	if cfg.subnetContext == nil {
		t.Fatal("single rule with subnet: subnetContext should be auto-built")
	}
	if cfg.subnetContext.Count() != 1 {
		t.Fatalf("single rule with subnet: count=%d, want 1", cfg.subnetContext.Count())
	}

	// Multiple rules with different subnets
	args = append(base,
		"rule=subnet:10.40.1.0/24,type:Node,hostname:compute-{04d},routers:10.40.1.1,cidr:24",
		"rule=subnet:10.40.3.0/24,type:Node,hostname:storage-{04d},routers:10.40.3.1,cidr:24",
	)
	cfg, errs = parseConfig(args...)
	if len(errs) != 0 {
		t.Fatalf("multiple rules with subnets: unexpected errors: %v", errs)
	}
	if cfg.subnetContext == nil {
		t.Fatal("multiple rules with subnets: subnetContext should be auto-built")
	}
	if cfg.subnetContext.Count() != 2 {
		t.Fatalf("multiple rules with subnets: count=%d, want 2", cfg.subnetContext.Count())
	}

	// Duplicate subnet across rules should not double-count
	args = append(base,
		"rule=subnet:10.40.1.0/24,type:Node,hostname:compute-{04d},routers:10.40.1.1,cidr:24",
		"rule=subnet:10.40.1.0/24,type:NodeBMC,hostname:bmc-{04d}",
	)
	cfg, errs = parseConfig(args...)
	if len(errs) != 0 {
		t.Fatalf("duplicate subnet rules: unexpected errors: %v", errs)
	}
	if cfg.subnetContext.Count() != 1 {
		t.Fatalf("duplicate subnet rules: count=%d, want 1", cfg.subnetContext.Count())
	}

	// Rules without subnet match should not create SubnetContext
	args = append(base,
		"rule=type:Node,hostname:nid{04d}",
		"rule=type:NodeBMC,hostname:bmc{04d}",
	)
	cfg, errs = parseConfig(args...)
	if len(errs) != 0 {
		t.Fatalf("rules without subnet: unexpected errors: %v", errs)
	}
	if cfg.subnetContext != nil {
		t.Fatal("rules without subnet: subnetContext should be nil")
	}
}

func TestParseConfig_SubnetWithRules(t *testing.T) {
	args := []string{
		"svc_base_uri=https://svc.example.test",
		"ipxe_base_uri=https://ipxe.example.test",
		"rule=subnet:10.40.1.0/24,type:Node,hostname:compute-{04d},routers:10.40.1.1,cidr:24",
		"rule=subnet:10.40.3.0/24,type:Node,hostname:storage-{04d},routers:10.40.3.1,cidr:24",
		"rule=type:NodeBMC,hostname:bmc{04d}",
	}

	cfg, errs := parseConfig(args...)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if cfg.subnetContext == nil || cfg.subnetContext.Count() != 2 {
		t.Fatalf("expected 2 subnets in context, got %v", cfg.subnetContext)
	}
	if len(cfg.rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(cfg.rules))
	}
	// Verify first rule has subnet match and router action
	r := cfg.rules[0]
	if len(r.Match.Subnets) != 1 {
		t.Fatalf("rule[0] expected 1 subnet match, got %d", len(r.Match.Subnets))
	}
	if len(r.Action.Routers) != 1 {
		t.Fatalf("rule[0] expected 1 router, got %d", len(r.Action.Routers))
	}
	if ones, _ := r.Action.Netmask.Size(); ones != 24 {
		t.Fatalf("rule[0] expected /24 netmask, got /%d", ones)
	}
}

func TestSetup6_InvalidConfigFails(t *testing.T) {
	if Plugin.Setup6 == nil {
		t.Fatal("Plugin.Setup6 is nil")
	}
	h, err := Plugin.Setup6()
	if err == nil {
		t.Fatalf("setup6() with no args: expected error")
	}
	if h != nil {
		t.Fatalf("setup6() with invalid config: expected nil handler")
	}
}
