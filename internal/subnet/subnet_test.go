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

// --- SubnetPoolManager tests ---

func TestNewSubnetPoolManager(t *testing.T) {
	spm := NewSubnetPoolManager()
	if spm == nil {
		t.Fatal("NewSubnetPoolManager() returned nil")
	}
	if !spm.IsEmpty() {
		t.Error("NewSubnetPoolManager() should be empty")
	}
	if spm.Count() != 0 {
		t.Errorf("Count() = %d, want 0", spm.Count())
	}
}

func TestSubnetPoolManager_AddPool(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		startIP   string
		endIP     string
		wantError bool
	}{
		{
			name:      "valid pool",
			cidr:      "10.40.1.0/24",
			startIP:   "10.40.1.10",
			endIP:     "10.40.1.200",
			wantError: false,
		},
		{
			name:      "invalid CIDR",
			cidr:      "invalid",
			startIP:   "10.40.1.10",
			endIP:     "10.40.1.200",
			wantError: true,
		},
		{
			name:      "start IP outside subnet",
			cidr:      "10.40.1.0/24",
			startIP:   "10.40.2.10",
			endIP:     "10.40.1.200",
			wantError: true,
		},
		{
			name:      "end IP outside subnet",
			cidr:      "10.40.1.0/24",
			startIP:   "10.40.1.10",
			endIP:     "10.40.2.200",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spm := NewSubnetPoolManager()
			err := spm.AddPool(tt.cidr, net.ParseIP(tt.startIP), net.ParseIP(tt.endIP))
			if (err != nil) != tt.wantError {
				t.Errorf("AddPool() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && spm.Count() != 1 {
				t.Errorf("Count() = %d, want 1 after successful AddPool", spm.Count())
			}
		})
	}
}

func TestSubnetPoolManager_GetAllocatorForGiaddr(t *testing.T) {
	spm := NewSubnetPoolManager()
	spm.AddPool("10.40.1.0/24", net.ParseIP("10.40.1.10"), net.ParseIP("10.40.1.200"))
	spm.AddPool("10.40.3.0/24", net.ParseIP("10.40.3.10"), net.ParseIP("10.40.3.200"))

	tests := []struct {
		name      string
		giaddr    string
		wantCIDR  string
		wantError bool
	}{
		{
			name:      "giaddr in first subnet",
			giaddr:    "10.40.1.1",
			wantCIDR:  "10.40.1.0/24",
			wantError: false,
		},
		{
			name:      "giaddr in second subnet",
			giaddr:    "10.40.3.1",
			wantCIDR:  "10.40.3.0/24",
			wantError: false,
		},
		{
			name:      "giaddr not in any subnet",
			giaddr:    "192.168.1.1",
			wantError: true,
		},
		{
			name:      "unspecified giaddr with multiple pools errors",
			giaddr:    "0.0.0.0",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc, cidr, err := spm.GetAllocatorForGiaddr(net.ParseIP(tt.giaddr))
			if (err != nil) != tt.wantError {
				t.Errorf("GetAllocatorForGiaddr() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				return
			}
			if alloc == nil {
				t.Error("GetAllocatorForGiaddr() returned nil allocator")
			}
			if cidr != tt.wantCIDR {
				t.Errorf("GetAllocatorForGiaddr() cidr = %v, want %v", cidr, tt.wantCIDR)
			}
		})
	}
}

func TestSubnetPoolManager_GetAllocatorForGiaddr_SinglePoolFallback(t *testing.T) {
	spm := NewSubnetPoolManager()
	spm.AddPool("10.40.1.0/24", net.ParseIP("10.40.1.10"), net.ParseIP("10.40.1.200"))

	// With a single pool and unspecified giaddr, fallback to that pool
	alloc, cidr, err := spm.GetAllocatorForGiaddr(net.IPv4zero)
	if err != nil {
		t.Fatalf("expected fallback to single pool, got error: %v", err)
	}
	if alloc == nil {
		t.Fatal("expected non-nil allocator")
	}
	if cidr != "10.40.1.0/24" {
		t.Fatalf("expected cidr=10.40.1.0/24, got %s", cidr)
	}
}

func TestSubnetPoolManager_GetAllocatorForSubnet(t *testing.T) {
	spm := NewSubnetPoolManager()
	spm.AddPool("10.40.1.0/24", net.ParseIP("10.40.1.10"), net.ParseIP("10.40.1.200"))

	alloc, err := spm.GetAllocatorForSubnet("10.40.1.0/24")
	if err != nil {
		t.Fatalf("GetAllocatorForSubnet() unexpected error: %v", err)
	}
	if alloc == nil {
		t.Fatal("GetAllocatorForSubnet() returned nil allocator")
	}

	_, err = spm.GetAllocatorForSubnet("10.40.99.0/24")
	if err == nil {
		t.Fatal("GetAllocatorForSubnet() expected error for unknown subnet")
	}
}

func TestSubnetPoolManager_IsEmptyAndCount(t *testing.T) {
	spm := NewSubnetPoolManager()
	if !spm.IsEmpty() {
		t.Error("IsEmpty() should return true for new manager")
	}
	if spm.Count() != 0 {
		t.Errorf("Count() = %d, want 0", spm.Count())
	}

	spm.AddPool("10.40.1.0/24", net.ParseIP("10.40.1.10"), net.ParseIP("10.40.1.200"))
	if spm.IsEmpty() {
		t.Error("IsEmpty() should return false after adding pool")
	}
	if spm.Count() != 1 {
		t.Errorf("Count() = %d, want 1", spm.Count())
	}
}

// --- SubnetContext tests ---

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
