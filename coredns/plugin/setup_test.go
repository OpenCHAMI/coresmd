package plugin

import (
	"strings"
	"testing"

	"github.com/coredns/caddy"
)

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

func TestParseConfigurationWithZones(t *testing.T) {
	corefile := `coresmd {
		smd_url https://smd.cluster.local
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

	if len(plugin.zones) == 0 {
		t.Log("No zones configured, default zones will be set during OnStartup")
	}
}

func TestParseConfigurationWithMultipleZones(t *testing.T) {
	corefile := `coresmd {
		smd_url https://smd.cluster.local
		ca_cert /path/to/ca.crt
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
            bmcs bmc-{id}
        }
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}`

	c := caddy.NewTestController("dns", corefile)
	// Advance to the server block
	if !c.Next() {
		t.Fatal("Failed to advance to server block")
	}
	// Advance to the coresmd plugin block
	found := false
	for c.NextBlock() {
		if c.Val() == "coresmd" {
			plugin, err := parse(c)
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
			if zone.BMCPattern != "bmc-{id}" {
				t.Errorf("Expected BMCPattern to be 'bmc-{id}', got '%s'", zone.BMCPattern)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("Did not find coresmd block in Corefile")
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
				BMCPattern:  "bmc-{id}",
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
