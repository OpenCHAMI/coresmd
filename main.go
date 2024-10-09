package coresmd

import (
	"errors"
	"fmt"
	"net"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/coresmd")

var Plugin = plugins.Plugin{
	Name:   "coresmd",
	Setup6: setup6,
	Setup4: setup4,
}

var cache *Cache

func setup6(args ...string) (handler.Handler6, error) {
	return nil, errors.New("coresmd does not currently support DHCPv6")
}

func setup4(args ...string) (handler.Handler4, error) {
	// Ensure all required args were passed
	if len(args) != 2 {
		return nil, errors.New("expected 2 arguments: base URL, cache duration")
	}

	// Create new SmdClient using first argument (base URL)
	log.Debug("generating new SmdClient")
	smdClient, err := NewSmdClient(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to create new SMD client: %w", err)
	}

	// Create new Cache using second argument (cache validity duration) and new SmdClient
	// pointer
	log.Debug("generating new Cache")
	cache, err := NewCache(args[1], smdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create new cache: %w", err)
	}

	cache.RefreshLoop()

	log.Infof("coresmd plugin initialized with base URL %s and validity duration %s", smdClient.BaseURL, cache.Duration.String())

	return Handler4, nil
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	cache.Mutex.RLock()
	defer cache.Mutex.RUnlock()

	hwAddr := req.ClientHWAddr.String()
	if ei, ok := cache.EthernetInterfaces[hwAddr]; ok {
		compId := ei.ComponentID
		log.Debugf("EthernetInterface found in cache for hardware address %s with ID %s", hwAddr, compId)
		if _, ok := cache.Components[compId]; !ok {
			log.Errorf("no Component %s found in cache for EthernetInterface hardware address %s", compId, hwAddr)
			return resp, true
		}
		log.Debugf("Component found in cache with matching ID %s", compId)
		if len(ei.IPAddresses) == 0 {
			log.Errorf("no IP addresses found for component %s with hardware address %s", compId, hwAddr)
			return resp, true
		}
		log.Debugf("IP addresses available for hardware address %s (component %s): %v", hwAddr, compId, ei.IPAddresses)
		ip := ei.IPAddresses[0].IPAddress
		log.Infof("setting IP for %s to %s", hwAddr, ip)
		resp.YourIPAddr = net.ParseIP(ip)
		return resp, false
	}

	log.Infof("no EthernetInterfaces were found in cache for hardware address %s", hwAddr)
	return resp, true
}
