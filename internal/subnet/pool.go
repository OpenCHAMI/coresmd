// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package subnet

import (
	"fmt"
	"net"

	"github.com/coredhcp/coredhcp/plugins/allocators"
	"github.com/coredhcp/coredhcp/plugins/allocators/bitmap"
)

// SubnetPool represents an IP pool for a specific subnet
type SubnetPool struct {
	CIDR      *net.IPNet
	Allocator allocators.Allocator
}

// SubnetPoolManager manages multiple IP pools, one per subnet
type SubnetPoolManager struct {
	Pools map[string]*SubnetPool // key: CIDR string
}

// NewSubnetPoolManager creates a new SubnetPoolManager
func NewSubnetPoolManager() *SubnetPoolManager {
	return &SubnetPoolManager{
		Pools: make(map[string]*SubnetPool),
	}
}

// AddPool adds an IP pool for a specific subnet
func (spm *SubnetPoolManager) AddPool(cidr string, startIP, endIP net.IP) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// Verify IPs are within the subnet
	if !ipnet.Contains(startIP) {
		return fmt.Errorf("start IP %s is not within subnet %s", startIP, cidr)
	}
	if !ipnet.Contains(endIP) {
		return fmt.Errorf("end IP %s is not within subnet %s", endIP, cidr)
	}

	// Create allocator for this subnet
	allocator, err := bitmap.NewIPv4Allocator(startIP, endIP)
	if err != nil {
		return fmt.Errorf("failed to create allocator for subnet %s: %w", cidr, err)
	}

	spm.Pools[cidr] = &SubnetPool{
		CIDR:      ipnet,
		Allocator: allocator,
	}

	return nil
}

// GetAllocatorForGiaddr returns the allocator for the subnet containing giaddr
func (spm *SubnetPoolManager) GetAllocatorForGiaddr(giaddr net.IP) (allocators.Allocator, string, error) {
	if giaddr == nil || giaddr.IsUnspecified() {
		// No giaddr, try to use the first pool (backward compatibility)
		if len(spm.Pools) == 1 {
			for cidr, pool := range spm.Pools {
				return pool.Allocator, cidr, nil
			}
		}
		return nil, "", fmt.Errorf("giaddr is nil or unspecified and multiple pools configured")
	}

	// Find the pool that contains giaddr
	for cidr, pool := range spm.Pools {
		if pool.CIDR.Contains(giaddr) {
			return pool.Allocator, cidr, nil
		}
	}

	return nil, "", fmt.Errorf("no pool found for giaddr %s", giaddr.String())
}

// GetAllocatorForSubnet returns the allocator for a specific subnet CIDR
func (spm *SubnetPoolManager) GetAllocatorForSubnet(cidr string) (allocators.Allocator, error) {
	pool, ok := spm.Pools[cidr]
	if !ok {
		return nil, fmt.Errorf("no pool found for subnet %s", cidr)
	}
	return pool.Allocator, nil
}

// Count returns the number of configured pools
func (spm *SubnetPoolManager) Count() int {
	return len(spm.Pools)
}

// IsEmpty returns true if no pools are configured
func (spm *SubnetPoolManager) IsEmpty() bool {
	return len(spm.Pools) == 0
}
