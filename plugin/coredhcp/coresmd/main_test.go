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

func TestParseConfig_AuthSettings(t *testing.T) {
	cfg, errs := parseConfig(
		"svc_base_uri=https://svc.example.test",
		"ipxe_base_uri=https://ipxe.example.test",
		"auth_mode=optional",
		"tokensmith_url=https://tokensmith.example.test",
		"refresh_before=75s",
	)
	if len(errs) != 0 {
		t.Fatalf("parseConfig() unexpected errors: %v", errs)
	}
	if cfg.authMode != "optional" {
		t.Fatalf("authMode=%q, want %q", cfg.authMode, "optional")
	}
	if cfg.tokensmithURL != "https://tokensmith.example.test" {
		t.Fatalf("tokensmithURL=%q", cfg.tokensmithURL)
	}
	if cfg.refreshBefore == nil || cfg.refreshBefore.String() != "1m15s" {
		t.Fatalf("refreshBefore=%v, want 1m15s", cfg.refreshBefore)
	}
}

func TestParseConfig_InvalidAuthInputs(t *testing.T) {
	_, errs := parseConfig(
		"svc_base_uri=https://svc.example.test",
		"ipxe_base_uri=https://ipxe.example.test",
		"auth_mode=shadow",
		"refresh_before=not-a-duration",
	)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}

	hasModeErr := false
	hasRefreshErr := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "unknown auth_mode") {
			hasModeErr = true
		}
		if strings.Contains(err.Error(), "invalid duration") {
			hasRefreshErr = true
		}
	}
	if !hasModeErr {
		t.Fatalf("expected unknown auth_mode error, got: %v", errs)
	}
	if !hasRefreshErr {
		t.Fatalf("expected invalid refresh_before duration error, got: %v", errs)
	}
}

func TestConfigValidate_AuthRequiresTokensmithURL(t *testing.T) {
	svc, _ := url.Parse("https://svc.example.test")
	ipxe, _ := url.Parse("https://ipxe.example.test")

	cfg := Config{svcBaseURI: svc, ipxeBaseURI: ipxe, authMode: "required"}
	_, errs := cfg.validate()
	if len(errs) == 0 {
		t.Fatal("expected validation error when auth_mode is required and tokensmith_url is missing")
	}

	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "tokensmith_url is required when auth_mode") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing tokensmith_url validation error, got: %v", errs)
	}
}

func TestConfigValidate_DisabledAuthDoesNotRequireTokensmithURL(t *testing.T) {
	svc, _ := url.Parse("https://svc.example.test")
	ipxe, _ := url.Parse("https://ipxe.example.test")

	cfg := Config{svcBaseURI: svc, ipxeBaseURI: ipxe, authMode: ""}
	_, errs := cfg.validate()
	for _, err := range errs {
		if strings.Contains(err.Error(), "tokensmith_url is required when auth_mode") {
			t.Fatalf("did not expect tokensmith_url error for disabled/zero auth mode: %v", errs)
		}
	}
}
