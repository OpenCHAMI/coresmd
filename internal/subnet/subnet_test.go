// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package subnet

import (
	"net"
	"testing"
)

func TestNewSubnetContext(t *testing.T) {
	sc := NewSubnetContext()
	if sc == nil {
		t.Fatal("NewSubnetContext() returned nil")
	}
	if sc.Subnets == nil {
		t.Fatal("NewSubnetContext() did not initialize Subnets map")
	}
	if !sc.IsEmpty() {
		t.Error("NewSubnetContext() should be empty")
	}
}

func TestAddSubnet(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		router    string
		wantError bool
	}{
		{
			name:      "valid subnet",
			cidr:      "10.40.1.0/24",
			router:    "10.40.1.1",
			wantError: false,
		},
		{
			name:      "invalid CIDR",
			cidr:      "invalid",
			router:    "10.40.1.1",
			wantError: true,
		},
		{
			name:      "invalid router IP",
			cidr:      "10.40.1.0/24",
			router:    "invalid",
			wantError: true,
		},
		{
			name:      "router outside subnet",
			cidr:      "10.40.1.0/24",
			router:    "10.40.2.1",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSubnetContext()
			err := sc.AddSubnet(tt.cidr, tt.router)
			if (err != nil) != tt.wantError {
				t.Errorf("AddSubnet() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestFindSubnetForIP(t *testing.T) {
	sc := NewSubnetContext()
	sc.AddSubnet("10.40.1.0/24", "10.40.1.1")
	sc.AddSubnet("10.40.3.0/24", "10.40.3.1")

	tests := []struct {
		name      string
		ip        string
		wantCIDR  string
		wantError bool
	}{
		{
			name:      "IP in first subnet",
			ip:        "10.40.1.50",
			wantCIDR:  "10.40.1.0/24",
			wantError: false,
		},
		{
			name:      "IP in second subnet",
			ip:        "10.40.3.100",
			wantCIDR:  "10.40.3.0/24",
			wantError: false,
		},
		{
			name:      "IP not in any subnet",
			ip:        "192.168.1.1",
			wantCIDR:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			_, cidr, err := sc.FindSubnetForIP(ip)
			if (err != nil) != tt.wantError {
				t.Errorf("FindSubnetForIP() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && cidr != tt.wantCIDR {
				t.Errorf("FindSubnetForIP() cidr = %v, want %v", cidr, tt.wantCIDR)
			}
		})
	}
}

func TestMatchInterfaceToSubnet(t *testing.T) {
	sc := NewSubnetContext()
	sc.AddSubnet("10.40.1.0/24", "10.40.1.1")
	sc.AddSubnet("10.40.3.0/24", "10.40.3.1")

	tests := []struct {
		name      string
		ifaceIP   string
		giaddr    string
		wantMatch bool
	}{
		{
			name:      "matching subnet",
			ifaceIP:   "10.40.1.50",
			giaddr:    "10.40.1.1",
			wantMatch: true,
		},
		{
			name:      "non-matching subnet",
			ifaceIP:   "10.40.1.50",
			giaddr:    "10.40.3.1",
			wantMatch: false,
		},
		{
			name:      "no giaddr (direct)",
			ifaceIP:   "10.40.1.50",
			giaddr:    "0.0.0.0",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ifaceIP := net.ParseIP(tt.ifaceIP)
			giaddr := net.ParseIP(tt.giaddr)
			match := sc.MatchInterfaceToSubnet(ifaceIP, giaddr)
			if match != tt.wantMatch {
				t.Errorf("MatchInterfaceToSubnet() = %v, want %v", match, tt.wantMatch)
			}
		})
	}
}

func TestGetRouterForSubnet(t *testing.T) {
	sc := NewSubnetContext()
	sc.AddSubnet("10.40.1.0/24", "10.40.1.1")

	tests := []struct {
		name       string
		cidr       string
		wantRouter string
		wantError  bool
	}{
		{
			name:       "existing subnet",
			cidr:       "10.40.1.0/24",
			wantRouter: "10.40.1.1",
			wantError:  false,
		},
		{
			name:       "non-existing subnet",
			cidr:       "10.40.2.0/24",
			wantRouter: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := sc.GetRouterForSubnet(tt.cidr)
			if (err != nil) != tt.wantError {
				t.Errorf("GetRouterForSubnet() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && router.String() != tt.wantRouter {
				t.Errorf("GetRouterForSubnet() = %v, want %v", router, tt.wantRouter)
			}
		})
	}
}

func TestAddSubnetCIDROnly(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		wantError bool
	}{
		{
			name:      "valid CIDR",
			cidr:      "10.40.1.0/24",
			wantError: false,
		},
		{
			name:      "invalid CIDR",
			cidr:      "invalid",
			wantError: true,
		},
		{
			name:      "valid /21 CIDR",
			cidr:      "172.16.0.0/21",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSubnetContext()
			err := sc.AddSubnetCIDROnly(tt.cidr)
			if (err != nil) != tt.wantError {
				t.Errorf("AddSubnetCIDROnly() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError {
				if sc.Count() != 1 {
					t.Errorf("Count() = %d, want 1", sc.Count())
				}
				// Router should be nil
				config := sc.Subnets[tt.cidr]
				if config == nil {
					t.Fatal("subnet config is nil")
				}
				if config.Router != nil {
					t.Errorf("expected nil router, got %v", config.Router)
				}
			}
		})
	}
}

func TestIsEmptyAndCount(t *testing.T) {
	sc := NewSubnetContext()
	if !sc.IsEmpty() {
		t.Error("IsEmpty() should return true for new context")
	}
	if sc.Count() != 0 {
		t.Errorf("Count() = %d, want 0", sc.Count())
	}

	sc.AddSubnet("10.40.1.0/24", "10.40.1.1")
	if sc.IsEmpty() {
		t.Error("IsEmpty() should return false after adding subnet")
	}
	if sc.Count() != 1 {
		t.Errorf("Count() = %d, want 1", sc.Count())
	}

	sc.AddSubnet("10.40.3.0/24", "10.40.3.1")
	if sc.Count() != 2 {
		t.Errorf("Count() = %d, want 2", sc.Count())
	}
}
