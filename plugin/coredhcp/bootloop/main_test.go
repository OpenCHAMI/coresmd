package bootloop

import (
	"net"
	"strings"
	"testing"
	"time"
)

//==============================================================================
// Helpers
//==============================================================================

func ipStrPtr(s string) *string {
	return &s
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

//==============================================================================
// Config.String Tests
//==============================================================================

func TestConfigString(t *testing.T) {
	ipStart := net.ParseIP("192.168.0.10")
	ipEnd := net.ParseIP("192.168.0.20")
	lease := 10 * time.Minute

	cfg := Config{
		leaseFile:  "/tmp/leases",
		leaseTime:  &lease,
		ipv4Start:  &ipStart,
		ipv4End:    &ipEnd,
		ipv4Range:  11,
		scriptPath: "/usr/local/bin/bootloop.sh",
	}

	got := cfg.String()
	wantSubstrings := []string{
		"ipv4_start=192.168.0.10",
		"ipv4_end=192.168.0.20",
		"ipv4_range=11",
		"script_path=/usr/local/bin/bootloop.sh",
	}

	for _, sub := range wantSubstrings {
		if !strings.Contains(got, sub) {
			t.Errorf("Config.String() = %q, expected to contain %q", got, sub)
		}
	}
}

//==============================================================================
// parseConfig Tests
//==============================================================================

func TestParseConfig(t *testing.T) {
	tenMinutes := 10 * time.Minute

	testGoodStartIP := net.ParseIP("192.168.0.10")
	testGoodEndIP := net.ParseIP("192.168.0.20")

	tests := []struct {
		name         string
		argv         []string
		want         Config
		wantErrCount int
		wantErrSub   []string
	}{
		{
			name: "empty arguments yields zero config and no errors",
			argv: nil,
			want: Config{},
		},
		{
			name: "valid full config",
			argv: []string{
				"lease_file=/tmp/bootloop.leases",
				"lease_time=10m",
				"ipv4_start=192.168.0.10",
				"ipv4_end=192.168.0.20",
				"script_path=/tmp/test.ipxe",
			},
			want: Config{
				leaseFile:  "/tmp/bootloop.leases",
				leaseTime:  &tenMinutes,
				ipv4Start:  &testGoodStartIP,
				ipv4End:    &testGoodEndIP,
				scriptPath: "/tmp/test.ipxe",
			},
		},
		{
			name: "invalid format without equal sign",
			argv: []string{"not_a_key_val"},
			want: Config{},
			// "arg 0: invalid format 'not_a_key_val', should be 'key=val'"
			wantErrCount: 1,
			wantErrSub:   []string{"invalid format 'not_a_key_val'"},
		},
		{
			name: "unknown key",
			argv: []string{"foo=bar"},
			want: Config{},
			// "unknown config key 'foo'"
			wantErrCount: 1,
			wantErrSub:   []string{"unknown config key 'foo'"},
		},
		{
			name: "empty lease_file rejected",
			argv: []string{"lease_file="},
			want: Config{},
			// "empty (skipping)"
			wantErrCount: 1,
			wantErrSub:   []string{"lease_file: empty"},
		},
		{
			name: "empty script_path sets default script and warns",
			argv: []string{`script_path=""`},
			want: Config{
				scriptPath: defaultScriptPath,
			},
			wantErrCount: 1,
			wantErrSub:   []string{"script_path: empty (setting to default script)"},
		},
		{
			name:         "invalid lease_time duration",
			argv:         []string{"lease_time=notaduration"},
			want:         Config{},
			wantErrCount: 1,
			wantErrSub:   []string{"invalid duration 'notaduration'"},
		},
		{
			name:         "invalid ipv4_start",
			argv:         []string{"ipv4_start=999.999.999.999"},
			want:         Config{},
			wantErrCount: 1,
			wantErrSub:   []string{"ipv4_start: invalid ip address"},
		},
		{
			name:         "invalid ipv4_end",
			argv:         []string{"ipv4_end=not_an_ip"},
			want:         Config{},
			wantErrCount: 1,
			wantErrSub:   []string{"ipv4_end: invalid ip address"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, errs := parseConfig(tt.argv...)

			// Check error count if specified
			if tt.wantErrCount >= 0 && len(errs) != tt.wantErrCount {
				t.Errorf("parseConfig() error count = %d, want %d, errors=%v",
					len(errs), tt.wantErrCount, errs)
			}

			// Check expected substrings in error messages
			for _, sub := range tt.wantErrSub {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), sub) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("parseConfig() errors = %v, expected one to contain %q", errs, sub)
				}
			}

			// Check leaseFile
			if got.leaseFile != tt.want.leaseFile {
				t.Errorf("parseConfig() leaseFile = %q, want %q", got.leaseFile, tt.want.leaseFile)
			}

			// Check leaseTime
			if (got.leaseTime == nil) != (tt.want.leaseTime == nil) {
				t.Fatalf("parseConfig() leaseTime nil mismatch: got=%v, want=%v", got.leaseTime, tt.want.leaseTime)
			}
			if got.leaseTime != nil && *got.leaseTime != *tt.want.leaseTime {
				t.Errorf("parseConfig() leaseTime = %v, want %v", *got.leaseTime, *tt.want.leaseTime)
			}

			// Check ipv4_start and ipv4_end by string
			var wantStartStr, wantEndStr string
			if tt.want.ipv4Start != nil {
				wantStartStr = tt.want.ipv4Start.String()
			}
			if tt.want.ipv4End != nil {
				wantEndStr = tt.want.ipv4End.String()
			}

			if tt.want.ipv4Start == nil {
				if got.ipv4Start != nil {
					t.Errorf("parseConfig() ipv4Start = %v, want nil", got.ipv4Start)
				}
			} else {
				if got.ipv4Start == nil {
					t.Fatalf("parseConfig() ipv4Start is nil, want %s", wantStartStr)
				}
				if got.ipv4Start.String() != wantStartStr {
					t.Errorf("parseConfig() ipv4Start = %s, want %s", got.ipv4Start.String(), wantStartStr)
				}
			}

			if tt.want.ipv4End == nil {
				if got.ipv4End != nil {
					t.Errorf("parseConfig() ipv4End = %v, want nil", got.ipv4End)
				}
			} else {
				if got.ipv4End == nil {
					t.Fatalf("parseConfig() ipv4End is nil, want %s", wantEndStr)
				}
				if got.ipv4End.String() != wantEndStr {
					t.Errorf("parseConfig() ipv4End = %s, want %s", got.ipv4End.String(), wantEndStr)
				}
			}

			// Check scriptPath
			if got.scriptPath != tt.want.scriptPath {
				t.Errorf("parseConfig() scriptPath = %q, want %q", got.scriptPath, tt.want.scriptPath)
			}
		})
	}
}

//==============================================================================
// Config.validate Tests
//==============================================================================

func TestConfigValidate(t *testing.T) {
	ipStart := net.ParseIP("192.168.0.10")
	ipEnd := net.ParseIP("192.168.0.20")
	ipStartHigh := net.ParseIP("192.168.0.20")
	ipEndLow := net.ParseIP("192.168.0.10")
	customLease := 15 * time.Minute
	defaultLease, _ := time.ParseDuration(defaultLeaseTime)

	tests := []struct {
		name           string
		cfg            Config
		wantWarnSub    []string
		wantErrSub     []string
		wantIPv4Range  uint32
		wantLeaseTime  *time.Duration
		wantScriptPath string
	}{
		{
			name: "valid config computes ipv4Range with no warnings or errors",
			cfg: Config{
				leaseFile:  "/tmp/leases",
				leaseTime:  &customLease,
				ipv4Start:  &ipStart,
				ipv4End:    &ipEnd,
				scriptPath: "/usr/local/bin/bootloop.sh",
			},
			wantWarnSub:    nil,
			wantErrSub:     nil,
			wantIPv4Range:  11, // 192.168.0.10 .. 192.168.0.20 inclusive
			wantLeaseTime:  &customLease,
			wantScriptPath: "/usr/local/bin/bootloop.sh",
		},
		{
			name: "missing lease_file is an error",
			cfg: Config{
				ipv4Start:  &ipStart,
				ipv4End:    &ipEnd,
				leaseTime:  &customLease,
				scriptPath: "/usr/local/bin/bootloop.sh",
			},
			wantErrSub:     []string{"lease_file is required"},
			wantScriptPath: "/usr/local/bin/bootloop.sh",
		},
		{
			name: "missing ipv4_start and ipv4_end both error",
			cfg: Config{
				leaseFile: "/tmp/leases",
				// both nil
			},
			wantErrSub: []string{
				"ipv4_start is required",
				"ipv4_end is required",
			},
		},
		{
			name: "invalid ip range start > end is error",
			cfg: Config{
				leaseFile: "/tmp/leases",
				ipv4Start: &ipStartHigh,
				ipv4End:   &ipEndLow,
			},
			wantErrSub: []string{
				"invalid range: ipv4_end",
			},
		},
		{
			name: "missing lease_time defaults and warns",
			cfg: Config{
				leaseFile: "/tmp/leases",
				ipv4Start: &ipStart,
				ipv4End:   &ipEnd,
			},
			wantWarnSub: []string{
				"lease_time unset, defaulting to",
			},
			wantLeaseTime: &defaultLease,
		},
		{
			name: "missing script_path defaults and warns",
			cfg: Config{
				leaseFile: "/tmp/leases",
				ipv4Start: &ipStart,
				ipv4End:   &ipEnd,
			},
			wantWarnSub: []string{
				"script_path unset, using default",
			},
			wantScriptPath: defaultScriptPath,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg // work on a copy
			warns, errs := cfg.validate()

			// Check that each wanted warning substring occurs in some warning
			for _, sub := range tt.wantWarnSub {
				found := false
				for _, w := range warns {
					if strings.Contains(w, sub) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validate() warns = %v, expected one to contain %q", warns, sub)
				}
			}

			// Check that each wanted error substring occurs in some error
			for _, sub := range tt.wantErrSub {
				found := false
				for _, e := range errs {
					if strings.Contains(e.Error(), sub) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validate() errs = %v, expected one to contain %q", errs, sub)
				}
			}

			// Check ipv4Range when expected > 0
			if tt.wantIPv4Range != 0 && cfg.ipv4Range != tt.wantIPv4Range {
				t.Errorf("validate() ipv4Range = %d, want %d", cfg.ipv4Range, tt.wantIPv4Range)
			}

			// Check leaseTime when specified
			if tt.wantLeaseTime != nil {
				if cfg.leaseTime == nil {
					t.Fatalf("validate() leaseTime is nil, want %v", *tt.wantLeaseTime)
				}
				if *cfg.leaseTime != *tt.wantLeaseTime {
					t.Errorf("validate() leaseTime = %v, want %v", *cfg.leaseTime, *tt.wantLeaseTime)
				}
			}

			// Check scriptPath when specified
			if tt.wantScriptPath != "" && cfg.scriptPath != tt.wantScriptPath {
				t.Errorf("validate() scriptPath = %q, want %q", cfg.scriptPath, tt.wantScriptPath)
			}
		})
	}
}
