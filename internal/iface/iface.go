// SPDX-FileCopyrightText: Copyright 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package iface

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"

	"github.com/openchami/coresmd/internal/cache"
	"github.com/openchami/coresmd/internal/subnet"
)

// IfaceInfo represents only the needed network information for an interface
// fetched from SMD.
type IfaceInfo struct {
	CompID  string
	CompNID int64
	Type    string
	MAC     string
	IPList  []net.IP
}

// LookupMAC takes a MAC address and returns an IfaceInfo that corresponds to
// network interface information for it in the cache fetched from SMD.
func LookupMAC(log *logrus.Entry, mac string, c *cache.Cache) (IfaceInfo, error) {
	if c == nil {
		return IfaceInfo{}, fmt.Errorf("cannot lookup MAC address from nil cache")
	}
	if log == nil {
		log = logrus.NewEntry(logrus.New())
	}

	var ii IfaceInfo

	// Match MAC address with EthernetInterface
	ei, ok := c.EthernetInterfaces[mac]
	if !ok {
		return ii, fmt.Errorf("no EthernetInterfaces were found in cache for hardware address %s", mac)
	}
	ii.MAC = mac

	// If found, make sure Component exists with ID matching to EthernetInterface ID
	ii.CompID = ei.ComponentID
	log.Debugf("EthernetInterface found in cache for hardware address %s with ID %s", ii.MAC, ii.CompID)
	comp, ok := c.Components[ii.CompID]
	if !ok {
		return ii, fmt.Errorf("no Component %s found in cache for EthernetInterface hardware address %s", ii.CompID, ii.MAC)
	}
	ii.Type = comp.Type
	log.Debugf("matching Component of type %s with ID %s found in cache for hardware address %s", ii.Type, ii.CompID, ii.MAC)
	if ii.Type == "Node" {
		ii.CompNID = comp.NID
	}
	if len(ei.IPAddresses) == 0 {
		return ii, fmt.Errorf("EthernetInterface for Component %s (type %s) contains no IP addresses for hardware address %s", ii.CompID, ii.Type, ii.MAC)
	}
	log.Debugf("IP addresses available for hardware address %s (Component %s of type %s): %v", ii.MAC, ii.CompID, ii.Type, ei.IPAddresses)
	var ipList []net.IP
	for _, ipStr := range ei.IPAddresses {
		ip := net.ParseIP(ipStr.IPAddress)
		ipList = append(ipList, ip)
	}
	ii.IPList = ipList

	return ii, nil
}

// LookupMACWithSubnet takes a MAC address and giaddr, and returns an IfaceInfo
// that corresponds to network interface information for it in the cache fetched
// from SMD. If subnet context is provided and giaddr is set, it will select the
// interface IP that belongs to the same subnet as giaddr.
func LookupMACWithSubnet(log *logrus.Entry, mac string, giaddr net.IP, c *cache.Cache, sc *subnet.SubnetContext) (IfaceInfo, error) {
	if c == nil {
		return IfaceInfo{}, fmt.Errorf("cannot lookup MAC address from nil cache")
	}
	if log == nil {
		log = logrus.NewEntry(logrus.New())
	}

	var ii IfaceInfo

	// Match MAC address with EthernetInterface
	ei, ok := c.EthernetInterfaces[mac]
	if !ok {
		return ii, fmt.Errorf("no EthernetInterfaces were found in cache for hardware address %s", mac)
	}
	ii.MAC = mac

	// If found, make sure Component exists with ID matching to EthernetInterface ID
	ii.CompID = ei.ComponentID
	log.Debugf("EthernetInterface found in cache for hardware address %s with ID %s", ii.MAC, ii.CompID)
	comp, ok := c.Components[ii.CompID]
	if !ok {
		return ii, fmt.Errorf("no Component %s found in cache for EthernetInterface hardware address %s", ii.CompID, ii.MAC)
	}
	ii.Type = comp.Type
	log.Debugf("matching Component of type %s with ID %s found in cache for hardware address %s", ii.Type, ii.CompID, ii.MAC)
	if ii.Type == "Node" {
		ii.CompNID = comp.NID
	}
	if len(ei.IPAddresses) == 0 {
		return ii, fmt.Errorf("EthernetInterface for Component %s (type %s) contains no IP addresses for hardware address %s", ii.CompID, ii.Type, ii.MAC)
	}
	log.Debugf("IP addresses available for hardware address %s (Component %s of type %s): %v", ii.MAC, ii.CompID, ii.Type, ei.IPAddresses)

	// Parse all IPs
	var allIPs []net.IP
	for _, ipStr := range ei.IPAddresses {
		ip := net.ParseIP(ipStr.IPAddress)
		if ip != nil {
			allIPs = append(allIPs, ip)
		}
	}

	// If subnet context is provided and giaddr is set, filter IPs by subnet
	if sc != nil && !sc.IsEmpty() && giaddr != nil && !giaddr.IsUnspecified() {
		log.Debugf("subnet-aware lookup: giaddr=%s, checking %d IPs", giaddr, len(allIPs))

		// Find the subnet that contains giaddr
		subnetConfig, cidr, err := sc.FindSubnetForIP(giaddr)
		if err != nil {
			log.Warnf("giaddr %s not in any configured subnet, using all IPs: %v", giaddr, err)
			ii.IPList = allIPs
			return ii, nil
		}

		log.Debugf("giaddr %s belongs to subnet %s", giaddr, cidr)

		// Filter IPs that belong to the same subnet
		var matchingIPs []net.IP
		for _, ip := range allIPs {
			if subnetConfig.CIDR.Contains(ip) {
				matchingIPs = append(matchingIPs, ip)
				log.Debugf("IP %s matches subnet %s", ip, cidr)
			} else {
				log.Debugf("IP %s does not match subnet %s", ip, cidr)
			}
		}

		if len(matchingIPs) == 0 {
			return ii, fmt.Errorf("no IP addresses for MAC %s match subnet %s (giaddr=%s)", mac, cidr, giaddr)
		}

		ii.IPList = matchingIPs
		log.Debugf("subnet-aware lookup selected %d IPs for MAC %s in subnet %s", len(matchingIPs), mac, cidr)
	} else {
		// No subnet context or no giaddr, use all IPs (backward compatible)
		ii.IPList = allIPs
		if sc == nil || sc.IsEmpty() {
			log.Debugf("no subnet context configured, using all %d IPs", len(allIPs))
		} else {
			log.Debugf("no giaddr specified, using all %d IPs", len(allIPs))
		}
	}

	return ii, nil
}
