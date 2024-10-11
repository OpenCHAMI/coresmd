package coresmd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

var log = logger.GetLogger("plugins/coresmd")

var Plugin = plugins.Plugin{
	Name:   "coresmd",
	Setup6: setup6,
	Setup4: setup4,
}

var cache *Cache
var baseURL *url.URL

func setup6(args ...string) (handler.Handler6, error) {
	return nil, errors.New("coresmd does not currently support DHCPv6")
}

func setup4(args ...string) (handler.Handler4, error) {
	// Ensure all required args were passed
	if len(args) != 4 {
		return nil, errors.New("expected 2 arguments: base URL, CA certificate path, cache duration, access token")
	}

	// Create new SmdClient using first argument (base URL)
	log.Debug("generating new SmdClient")
	var err error
	baseURL, err = url.Parse(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}
	smdClient := NewSmdClient(baseURL)

	// If nonempty, test that CA cert path exists (second argument)
	caCertPath := args[1]
	if caCertPath != "" {
		if err := smdClient.UseCACert(args[1]); err != nil {
			return nil, fmt.Errorf("failed to set CA certificate: %w", err)
		}
		log.Infof("set CA certificate for SMD to the contents of %s", caCertPath)
	} else {
		log.Infof("CA certificate path was empty, not setting")
	}

	// Create new Cache using third argument (cache validity duration) and new SmdClient
	// pointer
	log.Debug("generating new Cache")
	cache, err = NewCache(args[2], smdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create new cache: %w", err)
	}

	// Set access token using fourth argument
	accessToken = args[3]

	cache.RefreshLoop()

	log.Infof("coresmd plugin initialized with base URL %s and validity duration %s", smdClient.BaseURL, cache.Duration.String())

	return Handler4, nil
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	log.Debugf("HANDLER CALLED ON MESSAGE TYPE: req(%s), resp(%s)", req.MessageType(), resp.MessageType())
	log.Debugf("REQUEST: %s", req.Summary())

	(*cache).Mutex.RLock()
	defer cache.Mutex.RUnlock()

	// STEP 1: Assign IP address
	hwAddr := req.ClientHWAddr.String()
	if ei, ok := cache.EthernetInterfaces[hwAddr]; ok {
		// First, check EthernetInterfaces, which are mapped to Components
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

		// Set client IP address
		resp.YourIPAddr = net.ParseIP(ip)
	} else if rfe, ok := cache.RedfishEndpoints[hwAddr]; ok {
		// If not an EthernetInterface, check RedfishEndpoints which are attached to BMCs
		log.Debug("RedfishEndpoint found in cache for hardware address %s", hwAddr)
		ip := rfe.IPAddr
		log.Infof("setting IP for %s to %s", hwAddr, ip)

		// Set client IP address
		resp.YourIPAddr = net.ParseIP(ip)
	} else {
		log.Infof("no EthernetInterfaces or RedfishEndpoints were found in cache for hardware address %s", hwAddr)
		return resp, true
	}

	// STEP 2: Send boot config
	if cinfo := req.Options.Get(dhcpv4.OptionUserClassInformation); string(cinfo) != "iPXE" {
		// BOOT STAGE 1: Send iPXE bootloader over TFTP
		if req.Options.Has(dhcpv4.OptionClientSystemArchitectureType) {
			var carch iana.Arch
			carchBytes := req.Options.Get(dhcpv4.OptionClientSystemArchitectureType)
			log.Debugf("client architecture of %s is %v (%q)", hwAddr, carchBytes, string(carchBytes))
			carch = iana.Arch(binary.BigEndian.Uint16(carchBytes))
			switch carch {
			case iana.EFI_IA32:
				// iPXE legacy 32-bit x86 bootloader
				resp.Options.Update(dhcpv4.OptBootFileName("undionly.kpxe"))
			case iana.EFI_X86_64:
				// iPXE 64-bit x86 bootloader
				resp.Options.Update(dhcpv4.OptBootFileName("ipxe.efi"))
			default:
				log.Errorf("no iPXE bootloader available for unknown architecture: %d (%s)", carch, carch.String())
				return resp, true
			}
		} else {
			log.Errorf("client did not present an architecture, unable to provide correct iPXE bootloader")
			return resp, true
		}
	} else {
		// BOOT STAGE 2: Send URL to BSS boot script
		bssURL := bootScriptBaseURL.JoinPath("/boot/v1/bootscript")
		bssURL.RawQuery = fmt.Sprintf("mac=%s", hwAddr)
		resp.Options.Update(dhcpv4.OptBootFileName(bssURL.String()))
	}

	log.Debugf("resp (after): %v", resp)
	log.Debugf("RESPONSE: %s", resp.Summary())

	return resp, false
}
