package plugin

import (
	"strings"
	"testing"

	"github.com/coredns/caddy"
)

// parseCorefile is a generic test function that accepts a full Corefile as a multiline string
// and returns either an error (for unparsable files) or a plugin struct (for valid files).
// This allows for flexible testing of various Corefile configurations.
func parseCorefile(t *testing.T, corefile string) (*Plugin, error) {
	c := caddy.NewTestController("dns", corefile)

	// Advance to the server block
	if !c.Next() {
		return nil, c.Errf("failed to advance to server block")
	}

	// Look for the coresmd plugin block
	for c.NextBlock() {
		if c.Val() == "coresmd" {
			return parse(c)
		}
	}

	return nil, c.Errf("did not find coresmd block in Corefile")
}

func TestParseBasicConfiguration(t *testing.T) {
	corefile := `coresmd {
		smd_url https://smd.cluster.local
		ca_cert /path/to/ca.crt
		cache_duration 30s
	}`

	c := caddy.NewTestController("dns", corefile)
	plugin, err := parse(c)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if plugin == nil {
		t.Fatal("Expected plugin to be created")
	}

	if plugin.smdURL != "https://smd.cluster.local" {
		t.Errorf("Expected smd_url to be 'https://smd.cluster.local', got '%s'", plugin.smdURL)
	}

	if plugin.caCert != "/path/to/ca.crt" {
		t.Errorf("Expected ca_cert to be '/path/to/ca.crt', got '%s'", plugin.caCert)
	}

	if plugin.cacheDuration != "30s" {
		t.Errorf("Expected cache_duration to be '30s', got '%s'", plugin.cacheDuration)
	}
}

func TestParseConfigurationWithMultipleZones(t *testing.T) {
	corefile := `
.:1053 {
    coresmd {
        smd_url https://smd.cluster.local
        ca_cert /path/to/ca.crt
        zone openchami.cluster {
            nodes nid{04d}

        }
        zone internal.openchami.cluster {
            nodes nid{03d}

        }
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}`

	plugin, err := parseCorefile(t, corefile)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if plugin == nil {
		t.Fatal("Expected plugin to be created")
	}

	if plugin.smdURL != "https://smd.cluster.local" {
		t.Errorf("Expected smd_url to be 'https://smd.cluster.local', got '%s'", plugin.smdURL)
	}

	if plugin.caCert != "/path/to/ca.crt" {
		t.Errorf("Expected ca_cert to be '/path/to/ca.crt', got '%s'", plugin.caCert)
	}

	// Check that zones were parsed correctly
	if len(plugin.zones) != 2 {
		t.Fatalf("Expected 2 zones, got %d", len(plugin.zones))
	}

	// Check first zone
	zone1 := plugin.zones[0]
	if zone1.Name != "openchami.cluster" {
		t.Errorf("Expected first zone name to be 'openchami.cluster', got '%s'", zone1.Name)
	}
	if zone1.NodePattern != "nid{04d}" {
		t.Errorf("Expected first zone NodePattern to be 'nid{04d}', got '%s'", zone1.NodePattern)
	}

	// Check second zone
	zone2 := plugin.zones[1]
	if zone2.Name != "internal.openchami.cluster" {
		t.Errorf("Expected second zone name to be 'internal.openchami.cluster', got '%s'", zone2.Name)
	}
	if zone2.NodePattern != "nid{03d}" {
		t.Errorf("Expected second zone NodePattern to be 'nid{03d}', got '%s'", zone2.NodePattern)
	}
}

func TestParseConfigurationMissingSMDURL(t *testing.T) {
	corefile := `coresmd {
		ca_cert /path/to/ca.crt
		cache_duration 30s
	}`

	c := caddy.NewTestController("dns", corefile)
	_, err := parse(c)

	if err == nil {
		t.Fatal("Expected error for missing smd_url, got none")
	}

	if err.Error() != "smd_url is required" {
		t.Errorf("Expected error message 'smd_url is required', got '%s'", err.Error())
	}
}

func TestParseConfigurationDefaultCacheDuration(t *testing.T) {
	corefile := `coresmd {
		smd_url https://smd.cluster.local
	}`

	c := caddy.NewTestController("dns", corefile)
	plugin, err := parse(c)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if plugin.cacheDuration != "30s" {
		t.Errorf("Expected default cache_duration to be '30s', got '%s'", plugin.cacheDuration)
	}
}

func TestParseFullCorefileExample(t *testing.T) {
	corefile := `
.:1053 {
    coresmd {
        smd_url https://demo.openchami.cluster:8443
        cache_duration 30s
        zone openchami.cluster {
            nodes nid{04d}

        }
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}`

	plugin, err := parseCorefile(t, corefile)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if plugin == nil {
		t.Fatal("Expected plugin to be created")
	}

	if plugin.smdURL != "https://demo.openchami.cluster:8443" {
		t.Errorf("Expected smd_url to be 'https://demo.openchami.cluster:8443', got '%s'", plugin.smdURL)
	}

	if len(plugin.zones) != 1 {
		t.Fatalf("Expected 1 zone, got %d", len(plugin.zones))
	}

	zone := plugin.zones[0]
	if zone.Name != "openchami.cluster" {
		t.Errorf("Expected zone name to be 'openchami.cluster', got '%s'", zone.Name)
	}
	if zone.NodePattern != "nid{04d}" {
		t.Errorf("Expected NodePattern to be 'nid{04d}', got '%s'", zone.NodePattern)
	}

}

func TestParseConfigurationUnknownDirective(t *testing.T) {
	corefile := `coresmd {
		unknown_directive value
	}`

	c := caddy.NewTestController("dns", corefile)
	_, err := parse(c)

	if err == nil {
		t.Fatal("Expected error for unknown directive, got none")
	}

	if !strings.Contains(err.Error(), "unknown directive") {
		t.Errorf("Expected error to contain 'unknown directive', got '%s'", err.Error())
	}
}

func TestParseConfigurationMissingArgument(t *testing.T) {
	corefile := `coresmd {
		smd_url
	}`

	c := caddy.NewTestController("dns", corefile)
	_, err := parse(c)

	if err == nil {
		t.Fatal("Expected error for missing argument, got none")
	}
}

func TestPluginOnStartup(t *testing.T) {
	plugin := &Plugin{
		smdURL:        "https://smd.cluster.local",
		cacheDuration: "30s",
		zones: []Zone{
			{
				Name:        "cluster.local",
				NodePattern: "nid{04d}",
			},
		},
	}

	// Test that OnStartup doesn't panic
	err := plugin.OnStartup()
	if err != nil {
		t.Logf("OnStartup returned error (expected in test environment): %v", err)
	}
}

func TestPluginName(t *testing.T) {
	plugin := &Plugin{}
	if plugin.Name() != "coresmd" {
		t.Errorf("Expected plugin name to be 'coresmd', got '%s'", plugin.Name())
	}
}

// TestParseCorefileInvalidConfigurations demonstrates testing of invalid Corefile configurations
func TestParseCorefileInvalidConfigurations(t *testing.T) {
	testCases := []struct {
		name          string
		corefile      string
		expectError   bool
		errorContains string
	}{
		{
			name: "missing coresmd block",
			corefile: `
.:1053 {
    forward . 8.8.8.8
}`,
			expectError:   true,
			errorContains: "did not find coresmd block",
		},
		{
			name: "missing smd_url in full corefile",
			corefile: `
.:1053 {
    coresmd {
        ca_cert /path/to/ca.crt
    }
    forward . 8.8.8.8
}`,
			expectError:   true,
			errorContains: "smd_url is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCorefile(t, tc.corefile)
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got none")
				}
				if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestParseCorefileValidConfigurations demonstrates testing of various valid Corefile configurations
func TestParseCorefileValidConfigurations(t *testing.T) {
	testCases := []struct {
		name     string
		corefile string
		validate func(*testing.T, *Plugin)
	}{
		{
			name: "minimal configuration",
			corefile: `
.:1053 {
    coresmd {
        smd_url https://smd.cluster.local
    }
    forward . 8.8.8.8
}`,
			validate: func(t *testing.T, plugin *Plugin) {
				if plugin.smdURL != "https://smd.cluster.local" {
					t.Errorf("Expected smd_url to be 'https://smd.cluster.local', got '%s'", plugin.smdURL)
				}
				if plugin.cacheDuration != "30s" {
					t.Errorf("Expected default cache_duration to be '30s', got '%s'", plugin.cacheDuration)
				}
			},
		},
		{
			name: "full configuration with zones",
			corefile: `
.:1053 {
    coresmd {
        smd_url https://smd.cluster.local
        ca_cert /path/to/ca.crt
        cache_duration 60s
        zone cluster.local {
            nodes nid{04d}

        }
        zone test.local {
            nodes node{03d}

        }
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}`,
			validate: func(t *testing.T, plugin *Plugin) {
				if plugin.smdURL != "https://smd.cluster.local" {
					t.Errorf("Expected smd_url to be 'https://smd.cluster.local', got '%s'", plugin.smdURL)
				}
				if plugin.caCert != "/path/to/ca.crt" {
					t.Errorf("Expected ca_cert to be '/path/to/ca.crt', got '%s'", plugin.caCert)
				}
				if plugin.cacheDuration != "60s" {
					t.Errorf("Expected cache_duration to be '60s', got '%s'", plugin.cacheDuration)
				}
				if len(plugin.zones) != 2 {
					t.Fatalf("Expected 2 zones, got %d", len(plugin.zones))
				}

				// Check first zone
				zone1 := plugin.zones[0]
				if zone1.Name != "cluster.local" {
					t.Errorf("Expected first zone name to be 'cluster.local', got '%s'", zone1.Name)
				}
				if zone1.NodePattern != "nid{04d}" {
					t.Errorf("Expected first zone NodePattern to be 'nid{04d}', got '%s'", zone1.NodePattern)
				}

				// Check second zone
				zone2 := plugin.zones[1]
				if zone2.Name != "test.local" {
					t.Errorf("Expected second zone name to be 'test.local', got '%s'", zone2.Name)
				}
				if zone2.NodePattern != "node{03d}" {
					t.Errorf("Expected second zone NodePattern to be 'node{03d}', got '%s'", zone2.NodePattern)
				}

			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin, err := parseCorefile(t, tc.corefile)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if plugin == nil {
				t.Fatal("Expected plugin to be created")
			}
			tc.validate(t, plugin)
		})
	}
}

// debugParseCorefile is a helper function to debug what tokens the parser sees
func debugParseCorefile(t *testing.T, corefile string) {
	c := caddy.NewTestController("dns", corefile)

	t.Logf("=== Debug parsing for Corefile ===")
	t.Logf("Corefile:\n%s", corefile)

	// Advance to the server block
	if !c.Next() {
		t.Logf("Failed to advance to server block")
		return
	}
	t.Logf("Server block: %s", c.Val())

	// Look for the coresmd plugin block
	for c.NextBlock() {
		directive := c.Val()
		t.Logf("Directive: %q", directive)

		if directive == "coresmd" {
			t.Logf("Found coresmd block, entering parse()")
			plugin, err := parse(c)
			if err != nil {
				t.Logf("parse() returned error: %v", err)
			} else {
				t.Logf("parse() succeeded, plugin: %+v", plugin)
			}
			return
		}
	}

	t.Logf("Did not find coresmd block")
}

func TestDebugParseCorefile(t *testing.T) {
	corefile := `
.:1053 {
    coresmd {
        smd_url https://smd.cluster.local
        zone cluster.local {
            nodes nid{04d}

        }
        zone test.local {
            nodes node{03d}

        }
    }
    forward . 8.8.8.8
}`

	debugParseCorefile(t, corefile)
}
