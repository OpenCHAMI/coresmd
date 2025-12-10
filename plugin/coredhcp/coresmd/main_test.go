package coresmd

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestConfigString ensures String() includes the key fields in a stable format.
func TestConfigString(t *testing.T) {
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
	}

	s := cfg.String()

	// We don't assert exact formatting, just that the important pieces appear.
	wantSubstrings := []string{
		"svc_base_uri=" + svc.String(),
		"ipxe_base_uri=" + ipxe.String(),
		"ca_cert=/etc/ssl/ca.pem",
		"cache_valid=" + cacheDur.String(),
		"lease_time=" + leaseDur.String(),
		"single_port=true",
		"tftp_dir=/tftp",
		"tftp_port=1069",
	}

	for _, sub := range wantSubstrings {
		if !strings.Contains(s, sub) {
			t.Errorf("Config.String() = %q, expected to contain %q", s, sub)
		}
	}
}

// TestParseConfig_Table covers the various accepted config keys and error cases.
func TestParseConfig_Table(t *testing.T) {
	cacheDur := 15 * time.Second
	leaseDur := 30 * time.Minute

	tests := []struct {
		name        string
		args        []string
		wantCfg     func() Config
		wantErrsMin int // minimum number of errors expected (0 if none required)
	}{
		{
			name: "all valid values",
			args: []string{
				"svc_base_uri=https://svc.example.test",
				"ipxe_base_uri=https://ipxe.example.test",
				"ca_cert=/etc/pki/ca.pem",
				"cache_valid=" + cacheDur.String(),
				"lease_time=" + leaseDur.String(),
				"single_port=true",
				"tftp_dir=/tftp",
				"tftp_port=1069",
			},
			wantCfg: func() Config {
				svc, _ := url.Parse("https://svc.example.test")
				ipxe, _ := url.Parse("https://ipxe.example.test")
				return Config{
					svcBaseURI:  svc,
					ipxeBaseURI: ipxe,
					caCert:      "/etc/pki/ca.pem",
					cacheValid:  &cacheDur,
					leaseTime:   &leaseDur,
					singlePort:  true,
					tftpDir:     "/tftp",
					tftpPort:    1069,
				}
			},
			wantErrsMin: 0,
		},
		{
			name: "invalid arg format",
			args: []string{
				"svc_base_uri=https://svc.example.test",
				"badformat", // no '='
			},
			wantCfg: func() Config {
				svc, _ := url.Parse("https://svc.example.test")
				return Config{
					svcBaseURI: svc,
				}
			},
			wantErrsMin: 1,
		},
		{
			name: "invalid cache_valid duration",
			args: []string{
				"cache_valid=notaduration",
			},
			wantCfg:     func() Config { return Config{} },
			wantErrsMin: 1,
		},
		{
			name: "invalid lease_time duration",
			args: []string{
				"lease_time=notaduration",
			},
			wantCfg:     func() Config { return Config{} },
			wantErrsMin: 1,
		},
		{
			name: "invalid single_port value",
			args: []string{
				"single_port=notabool",
			},
			wantCfg: func() Config {
				// single_port should stay at the zero value (false) if parsing fails.
				return Config{}
			},
			wantErrsMin: 1,
		},
		{
			name: "invalid tftp_port non-integer",
			args: []string{
				"tftp_port=notanint",
			},
			wantCfg: func() Config {
				return Config{
					tftpPort: defaultTFTPPort,
				}
			},
			wantErrsMin: 1,
		},
		{
			name: "invalid tftp_port out of range",
			args: []string{
				"tftp_port=70000",
			},
			wantCfg: func() Config {
				return Config{
					tftpPort: defaultTFTPPort,
				}
			},
			wantErrsMin: 1,
		},
		{
			name: "unknown key produces error",
			args: []string{
				"svc_base_uri=https://svc.example.test",
				"unknown_key=value",
			},
			wantCfg: func() Config {
				svc, _ := url.Parse("https://svc.example.test")
				return Config{
					svcBaseURI: svc,
				}
			},
			wantErrsMin: 1,
		},
		{
			name: "tftp_dir trims quotes",
			args: []string{
				`tftp_dir="/quoted/path"`,
			},
			wantCfg: func() Config {
				return Config{
					tftpDir: "/quoted/path",
				}
			},
			wantErrsMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCfg, errs := parseConfig(tt.args...)
			if len(errs) < tt.wantErrsMin {
				t.Fatalf("parseConfig() errors = %d, want at least %d; errs=%v", len(errs), tt.wantErrsMin, errs)
			}

			wantCfg := tt.wantCfg()

			// Compare only the fields we care about for each test.
			if wantCfg.svcBaseURI != nil {
				if gotCfg.svcBaseURI == nil || wantCfg.svcBaseURI.String() != gotCfg.svcBaseURI.String() {
					t.Errorf("svcBaseURI = %v, want %v", gotCfg.svcBaseURI, wantCfg.svcBaseURI)
				}
			}
			if wantCfg.ipxeBaseURI != nil {
				if gotCfg.ipxeBaseURI == nil || wantCfg.ipxeBaseURI.String() != gotCfg.ipxeBaseURI.String() {
					t.Errorf("ipxeBaseURI = %v, want %v", gotCfg.ipxeBaseURI, wantCfg.ipxeBaseURI)
				}
			}
			if wantCfg.caCert != "" && gotCfg.caCert != wantCfg.caCert {
				t.Errorf("caCert = %q, want %q", gotCfg.caCert, wantCfg.caCert)
			}
			if wantCfg.cacheValid != nil {
				if gotCfg.cacheValid == nil || *gotCfg.cacheValid != *wantCfg.cacheValid {
					t.Errorf("cacheValid = %v, want %v", gotCfg.cacheValid, wantCfg.cacheValid)
				}
			}
			if wantCfg.leaseTime != nil {
				if gotCfg.leaseTime == nil || *gotCfg.leaseTime != *wantCfg.leaseTime {
					t.Errorf("leaseTime = %v, want %v", gotCfg.leaseTime, wantCfg.leaseTime)
				}
			}
			if gotCfg.singlePort != wantCfg.singlePort {
				t.Errorf("singlePort = %v, want %v", gotCfg.singlePort, wantCfg.singlePort)
			}
			if gotCfg.tftpDir != wantCfg.tftpDir {
				t.Errorf("tftpDir = %q, want %q", gotCfg.tftpDir, wantCfg.tftpDir)
			}
			if gotCfg.tftpPort != wantCfg.tftpPort {
				t.Errorf("tftpPort = %d, want %d", gotCfg.tftpPort, wantCfg.tftpPort)
			}
		})
	}
}

// TestConfigValidate_Table exercises validation, defaulting, and error paths.
func TestConfigValidate_Table(t *testing.T) {
	svc, _ := url.Parse("https://svc.example.test")
	ipxe, _ := url.Parse("https://ipxe.example.test")

	tests := []struct {
		name        string
		cfg         Config
		wantWarnMin int
		wantErrMin  int
		check       func(t *testing.T, cfg Config)
	}{
		{
			name:        "missing required URIs",
			cfg:         Config{},
			wantWarnMin: 1, // ca_cert / cache_valid / lease_time / tftp_* will warn
			wantErrMin:  2, // svc_base_uri and ipxe_base_uri required
			check:       func(t *testing.T, cfg Config) {},
		},
		{
			name: "valid URIs, defaults applied",
			cfg: Config{
				svcBaseURI:  svc,
				ipxeBaseURI: ipxe,
			},
			// Exact number of warnings depends on combinations; we only care that
			// defaults are applied and there are *some* warnings.
			wantWarnMin: 3,
			wantErrMin:  0,
			check: func(t *testing.T, cfg Config) {
				if cfg.cacheValid == nil || cfg.cacheValid.String() != defaultCacheValid {
					t.Errorf("cacheValid = %v, want %s", cfg.cacheValid, defaultCacheValid)
				}
				if cfg.leaseTime == nil || cfg.leaseTime.String() != defaultLeaseTime {
					t.Errorf("leaseTime = %v, want %s", cfg.leaseTime, defaultLeaseTime)
				}
				if cfg.tftpPort != defaultTFTPPort {
					t.Errorf("tftpPort = %d, want %d", cfg.tftpPort, defaultTFTPPort)
				}
				if cfg.tftpDir != defaultTFTPDirectory {
					t.Errorf("tftpDir = %q, want %q", cfg.tftpDir, defaultTFTPDirectory)
				}
			},
		},
		{
			name: "tftpPort negative then defaulted",
			cfg: Config{
				svcBaseURI:  svc,
				ipxeBaseURI: ipxe,
				tftpPort:    -1,
			},
			wantWarnMin: 1,
			wantErrMin:  0,
			check: func(t *testing.T, cfg Config) {
				if cfg.tftpPort != defaultTFTPPort {
					t.Errorf("tftpPort = %d, want %d", cfg.tftpPort, defaultTFTPPort)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Work on a copy because validate has a pointer receiver and mutates the config.
			cfgCopy := tt.cfg
			warns, errs := cfgCopy.validate()

			if len(warns) < tt.wantWarnMin {
				t.Errorf("validate() warnings = %d, want at least %d; warns=%v", len(warns), tt.wantWarnMin, warns)
			}
			if len(errs) < tt.wantErrMin {
				t.Errorf("validate() errors = %d, want at least %d; errs=%v", len(errs), tt.wantErrMin, errs)
			}

			if tt.check != nil {
				tt.check(t, cfgCopy)
			}
		})
	}
}

// TestSetup6_Unsupported ensures DHCPv6 is explicitly unsupported.
func TestSetup6_Unsupported(t *testing.T) {
	h, err := setup6()
	if h != nil {
		t.Errorf("setup6() handler = %v, want nil", h)
	}
	if err == nil {
		t.Fatalf("setup6() error = nil, want non-nil")
	}
}

// TestPluginMetadata ensures the Plugin descriptor is wired correctly.
func TestPluginMetadata(t *testing.T) {
	if Plugin.Name != "coresmd" {
		t.Errorf("Plugin.Name = %q, want %q", Plugin.Name, "coresmd")
	}
	if Plugin.Setup4 == nil {
		t.Error("Plugin.Setup4 is nil, want non-nil")
	}
	if Plugin.Setup6 == nil {
		t.Error("Plugin.Setup6 is nil, want non-nil")
	}
}
