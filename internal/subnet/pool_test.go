// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package subnet

import (
	"net"
	"testing"
)

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
		{
			name:      "start IP after end IP",
			cidr:      "10.40.1.0/24",
			startIP:   "10.40.1.200",
			endIP:     "10.40.1.10",
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
