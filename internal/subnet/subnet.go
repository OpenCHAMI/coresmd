// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package subnet

import (
	"fmt"
	"net"
)

// SubnetConfig represents a single subnet configuration with its CIDR and router
type SubnetConfig struct {
	CIDR   *net.IPNet
	Router net.IP
}

// SubnetContext provides subnet-aware context for DHCP operations
type SubnetContext struct {
	Subnets map[string]*SubnetConfig // key: CIDR string (e.g., "10.40.1.0/24")
}

// NewSubnetContext creates a new SubnetContext
func NewSubnetContext() *SubnetContext {
	return &SubnetContext{
		Subnets: make(map[string]*SubnetConfig),
	}
}

// AddSubnet adds a subnet configuration to the context
func (sc *SubnetContext) AddSubnet(cidr string, router string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	routerIP := net.ParseIP(router)
	if routerIP == nil {
		return fmt.Errorf("invalid router IP %s", router)
	}

	// Verify router is within the subnet
	if !ipnet.Contains(routerIP) {
		return fmt.Errorf("router IP %s is not within subnet %s", router, cidr)
	}

	sc.Subnets[cidr] = &SubnetConfig{
		CIDR:   ipnet,
		Router: routerIP,
	}

	return nil
}

// FindSubnetForIP finds the subnet configuration that contains the given IP
func (sc *SubnetContext) FindSubnetForIP(ip net.IP) (*SubnetConfig, string, error) {
	if ip == nil {
		return nil, "", fmt.Errorf("IP address is nil")
	}

	for cidr, config := range sc.Subnets {
		if config.CIDR.Contains(ip) {
			return config, cidr, nil
		}
	}

	return nil, "", fmt.Errorf("no subnet found for IP %s", ip.String())
}

// MatchInterfaceToSubnet checks if an interface IP belongs to a specific subnet
func (sc *SubnetContext) MatchInterfaceToSubnet(ifaceIP net.IP, giaddr net.IP) bool {
	if giaddr == nil || giaddr.IsUnspecified() {
		// No relay agent, match any interface
		return true
	}

	// Find the subnet that contains giaddr
	subnetConfig, _, err := sc.FindSubnetForIP(giaddr)
	if err != nil {
		return false
	}

	// Check if the interface IP is in the same subnet
	return subnetConfig.CIDR.Contains(ifaceIP)
}

// GetRouterForSubnet returns the router IP for a given subnet CIDR
func (sc *SubnetContext) GetRouterForSubnet(cidr string) (net.IP, error) {
	config, ok := sc.Subnets[cidr]
	if !ok {
		return nil, fmt.Errorf("subnet %s not found", cidr)
	}
	return config.Router, nil
}

// GetSubnetForGiaddr returns the subnet configuration for a given giaddr
func (sc *SubnetContext) GetSubnetForGiaddr(giaddr net.IP) (*SubnetConfig, string, error) {
	if giaddr == nil || giaddr.IsUnspecified() {
		return nil, "", fmt.Errorf("giaddr is nil or unspecified")
	}

	return sc.FindSubnetForIP(giaddr)
}

// IsEmpty returns true if no subnets are configured
func (sc *SubnetContext) IsEmpty() bool {
	return len(sc.Subnets) == 0
}

// Count returns the number of configured subnets
func (sc *SubnetContext) Count() int {
	return len(sc.Subnets)
}
