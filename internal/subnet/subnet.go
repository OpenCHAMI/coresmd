// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package subnet

import (
	"fmt"
	"net"
)

// SubnetConfig represents a single subnet configuration with its CIDR and router.
type SubnetConfig struct {
	// CIDR is the parsed subnet in CIDR notation (e.g., "10.40.1.0/24").
	CIDR *net.IPNet
	// Router is the router IP for this subnet (e.g., net.ParseIP("10.40.1.1")).
	// It may be nil when the subnet is auto-built from rule-level subnet: match
	// keys without an explicit router.
	Router net.IP
}

// SubnetContext holds the set of subnets the DHCP server is responsible for
// and provides lookup helpers used during packet processing. A map keyed by
// CIDR string (e.g., "10.40.1.0/24") is used so that subnet lookups by CIDR
// are O(1) and duplicate registration is trivially prevented.
type SubnetContext struct {
	// Subnets maps a CIDR string (e.g., "10.40.1.0/24") to its configuration.
	Subnets map[string]*SubnetConfig
}

// NewSubnetContext creates a new, empty SubnetContext with no subnets
// registered. Use AddSubnet or AddSubnetCIDROnly to populate it.
func NewSubnetContext() *SubnetContext {
	return &SubnetContext{
		Subnets: make(map[string]*SubnetConfig),
	}
}

// AddSubnet registers a subnet in the context.
//
//   - cidr: the subnet in CIDR notation, e.g. "10.40.1.0/24".
//   - router: the router IP for the subnet, e.g. "10.40.1.1".
//     Must be an address within cidr.
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

// AddSubnetCIDROnly adds a subnet to the context using only the CIDR (no router).
// This is used when building the SubnetContext from rule-level subnet: match
// keys, where the router is set separately via rule actions.
func (sc *SubnetContext) AddSubnetCIDROnly(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// Only add if not already present (avoid overwriting router-aware entries)
	if _, exists := sc.Subnets[cidr]; !exists {
		sc.Subnets[cidr] = &SubnetConfig{
			CIDR: ipnet,
		}
	}

	return nil
}

// FindSubnetForIP returns the SubnetConfig and CIDR string for the subnet
// that contains ip. Returns an error if ip is nil or no subnet matches.
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

// MatchInterfaceToSubnet reports whether ifaceIP belongs to the same subnet
// as giaddr. If giaddr is nil or unspecified (no relay agent), any interface
// is considered a match.
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

// GetSubnetForGiaddr returns the SubnetConfig and CIDR string for the subnet
// containing giaddr. Returns an error if giaddr is nil, unspecified, or not
// found in any registered subnet.
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
