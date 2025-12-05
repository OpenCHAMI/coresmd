package coresmd

import (
	"strings"
	"testing"
)

func TestParseSetup4Args_KeyValueFormat(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
		checkConfig func(*testing.T, *pluginConfig)
	}{
		{
			name: "minimal valid configuration",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *pluginConfig) {
				if cfg.SmdURL != "https://smd.cluster.local" {
					t.Errorf("expected SmdURL='https://smd.cluster.local', got '%s'", cfg.SmdURL)
				}
				if cfg.NodePattern != "nid{04d}" {
					t.Errorf("expected default NodePattern='nid{04d}', got '%s'", cfg.NodePattern)
				}
				if cfg.SinglePort != false {
					t.Errorf("expected default SinglePort=false, got %v", cfg.SinglePort)
				}
			},
		},
		{
			name: "full configuration with all optional params",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=24h",
				"single_port=true",
				"node_pattern=nid{04d}",
				"bmc_pattern=bmc{03d}",
				"domain=cluster.local",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *pluginConfig) {
				if cfg.BmcPattern != "bmc{03d}" {
					t.Errorf("expected BmcPattern='bmc{03d}', got '%s'", cfg.BmcPattern)
				}
				if cfg.Domain != "cluster.local" {
					t.Errorf("expected Domain='cluster.local', got '%s'", cfg.Domain)
				}
				if cfg.SinglePort != true {
					t.Errorf("expected SinglePort=true, got %v", cfg.SinglePort)
				}
			},
		},
		{
			name: "custom hostname patterns",
			args: []string{
				"smd_url=https://smd.dev-osc.lanl.gov",
				"boot_script_url=http://172.16.0.253:8081",
				"cache_duration=30s",
				"lease_duration=24h",
				"node_pattern=dev-s{02d}",
				"bmc_pattern=bmc{03d}",
				"domain=dev-osc.lanl.gov",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *pluginConfig) {
				if cfg.NodePattern != "dev-s{02d}" {
					t.Errorf("expected NodePattern='dev-s{02d}', got '%s'", cfg.NodePattern)
				}
				if cfg.Domain != "dev-osc.lanl.gov" {
					t.Errorf("expected Domain='dev-osc.lanl.gov', got '%s'", cfg.Domain)
				}
			},
		},
		{
			name: "order independent",
			args: []string{
				"lease_duration=1h",
				"cache_duration=30s",
				"boot_script_url=http://192.168.1.1",
				"smd_url=https://smd.cluster.local",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *pluginConfig) {
				if cfg.SmdURL != "https://smd.cluster.local" {
					t.Errorf("order-independent parsing failed for SmdURL")
				}
			},
		},
		{
			name: "missing required parameter smd_url",
			args: []string{
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
			},
			expectError: true,
			errorMsg:    "missing required parameter: smd_url",
		},
		{
			name: "missing required parameter boot_script_url",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"cache_duration=30s",
				"lease_duration=1h",
			},
			expectError: true,
			errorMsg:    "missing required parameter: boot_script_url",
		},
		{
			name: "missing required parameter cache_duration",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"lease_duration=1h",
			},
			expectError: true,
			errorMsg:    "missing required parameter: cache_duration",
		},
		{
			name: "missing required parameter lease_duration",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
			},
			expectError: true,
			errorMsg:    "missing required parameter: lease_duration",
		},
		{
			name: "invalid key=value format",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"invalid_argument_no_equals",
			},
			expectError: true,
			errorMsg:    "invalid argument format",
		},
		{
			name: "invalid single_port value",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"single_port=maybe",
			},
			expectError: true,
			errorMsg:    "invalid single_port value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := parseSetup4Args(tt.args...)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				} else if tt.checkConfig != nil {
					tt.checkConfig(t, config)
				}
			}
		})
	}
}

func TestParseSetup4Args_LegacyPositionalFormat(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
		checkConfig func(*testing.T, *pluginConfig)
	}{
		{
			name: "valid 6 argument legacy format",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"1h",
				"false",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *pluginConfig) {
				if cfg.SmdURL != "https://smd.cluster.local" {
					t.Errorf("expected SmdURL='https://smd.cluster.local', got '%s'", cfg.SmdURL)
				}
				if cfg.SinglePort != false {
					t.Errorf("expected SinglePort=false, got %v", cfg.SinglePort)
				}
				if cfg.NodePattern != "nid{04d}" {
					t.Errorf("expected default NodePattern='nid{04d}', got '%s'", cfg.NodePattern)
				}
			},
		},
		{
			name: "valid 9 argument legacy format with hostname patterns",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"24h",
				"false",
				"dev-s{02d}",
				"bmc{03d}",
				"dev-osc.lanl.gov",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *pluginConfig) {
				if cfg.NodePattern != "dev-s{02d}" {
					t.Errorf("expected NodePattern='dev-s{02d}', got '%s'", cfg.NodePattern)
				}
				if cfg.BmcPattern != "bmc{03d}" {
					t.Errorf("expected BmcPattern='bmc{03d}', got '%s'", cfg.BmcPattern)
				}
				if cfg.Domain != "dev-osc.lanl.gov" {
					t.Errorf("expected Domain='dev-osc.lanl.gov', got '%s'", cfg.Domain)
				}
			},
		},
		{
			name: "invalid single_port value in legacy format",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"1h",
				"maybe",
			},
			expectError: true,
			errorMsg:    "invalid single port toggle",
		},
		{
			name: "wrong number of arguments (5)",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"1h",
			},
			expectError: true,
			errorMsg:    "invalid arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := parseSetup4Args(tt.args...)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				} else if tt.checkConfig != nil {
					tt.checkConfig(t, config)
				}
			}
		})
	}
}

func TestParseSetup4Args_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty arguments",
			args:        []string{},
			expectError: true,
			errorMsg:    "invalid arguments",
		},
		{
			name: "key=value with empty value",
			args: []string{
				"smd_url=",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
			},
			expectError: false, // Empty string is valid, validation happens in setup4
		},
		{
			name: "key with no equals sign in mixed format",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url",
				"cache_duration=30s",
			},
			expectError: true,
			errorMsg:    "invalid argument format",
		},
		{
			name: "single_port with invalid boolean string",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"single_port=yes",
			},
			expectError: true,
			errorMsg:    "invalid single_port value",
		},
		{
			name: "single_port with numeric string",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"single_port=1",
			},
			expectError: false, // strconv.ParseBool accepts "1" as true
		},
		{
			name: "legacy format with 7 arguments",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"1h",
				"false",
				"extra",
			},
			expectError: true,
			errorMsg:    "invalid arguments",
		},
		{
			name: "legacy format with 8 arguments",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"1h",
				"false",
				"nid{04d}",
				"bmc{03d}",
			},
			expectError: true,
			errorMsg:    "invalid arguments",
		},
		{
			name: "legacy format with 10 arguments",
			args: []string{
				"https://smd.cluster.local",
				"http://192.168.1.1",
				"",
				"30s",
				"1h",
				"false",
				"nid{04d}",
				"bmc{03d}",
				"cluster.local",
				"extra",
			},
			expectError: true,
			errorMsg:    "invalid arguments",
		},
		{
			name: "legacy format with single argument",
			args: []string{
				"https://smd.cluster.local",
			},
			expectError: true,
			errorMsg:    "invalid arguments",
		},
		{
			name: "key=value format with unknown parameter",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"unknown_param=value",
			},
			expectError: false, // Unknown params are ignored (forward compatibility)
		},
		{
			name: "key=value with multiple equals signs",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"weird=key=with=equals",
			},
			expectError: false, // SplitN(2) handles this, value becomes "key=with=equals"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSetup4Args(tt.args...)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				} else {
					// Verify error message is helpful
					t.Logf("✓ Got helpful error: %s", err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestParseSetup4Args_ErrorMessages(t *testing.T) {
	// Test that error messages are clear and actionable
	tests := []struct {
		name             string
		args             []string
		expectError      bool
		errorContains    []string // All strings must be in error message
		errorDescription string
	}{
		{
			name: "missing smd_url has clear hint",
			args: []string{
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
			},
			expectError:      true,
			errorContains:    []string{"missing required parameter", "smd_url"},
			errorDescription: "should mention which parameter is missing",
		},
		{
			name: "invalid format shows expected format",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"noequals",
			},
			expectError:      true,
			errorContains:    []string{"invalid argument format", "noequals", "key=value"},
			errorDescription: "should show the problematic argument and expected format",
		},
		{
			name: "wrong argument count shows valid counts",
			args: []string{
				"arg1",
				"arg2",
				"arg3",
			},
			expectError:      true,
			errorContains:    []string{"invalid arguments", "6 or 9 args", "got 3 args"},
			errorDescription: "should explain valid argument counts",
		},
		{
			name: "invalid single_port shows valid values",
			args: []string{
				"smd_url=https://smd.cluster.local",
				"boot_script_url=http://192.168.1.1",
				"cache_duration=30s",
				"lease_duration=1h",
				"single_port=on",
			},
			expectError:      true,
			errorContains:    []string{"invalid single_port value", "on", "true", "false"},
			errorDescription: "should show invalid value and valid options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSetup4Args(tt.args...)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}

				errMsg := err.Error()
				for _, substr := range tt.errorContains {
					if !strings.Contains(errMsg, substr) {
						t.Errorf("error message missing '%s': %s\n  Description: %s",
							substr, errMsg, tt.errorDescription)
					}
				}
				t.Logf("✓ Error message: %s", errMsg)
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestExpandHostnamePattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		nid      int64
		id       string
		expected string
	}{
		{
			name:     "basic nid pattern",
			pattern:  "nid{04d}",
			nid:      1,
			id:       "x3000c0s0b0n0",
			expected: "nid0001",
		},
		{
			name:     "custom prefix with 2 digits",
			pattern:  "dev-s{02d}",
			nid:      5,
			id:       "x3000c0s0b0n5",
			expected: "dev-s05",
		},
		{
			name:     "bmc pattern with 3 digits",
			pattern:  "bmc{03d}",
			nid:      42,
			id:       "x3000c0s0b1",
			expected: "bmc042",
		},
		{
			name:     "xname id pattern",
			pattern:  "{id}",
			nid:      123,
			id:       "x3000c0s0b0n123",
			expected: "x3000c0s0b0n123",
		},
		{
			name:     "mixed pattern",
			pattern:  "node-{id}-{03d}",
			nid:      7,
			id:       "x1000c0s0b0n7",
			expected: "node-x1000c0s0b0n7-007",
		},
		{
			name:     "large nid with 5 digits",
			pattern:  "compute-{05d}",
			nid:      12345,
			id:       "x9999c0s0b0n0",
			expected: "compute-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandHostnamePattern(tt.pattern, tt.nid, tt.id)
			if result != tt.expected {
				t.Errorf("expandHostnamePattern(%q, %d, %q) = %q, want %q",
					tt.pattern, tt.nid, tt.id, result, tt.expected)
			}
		})
	}
}
