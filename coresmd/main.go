package coresmd

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/openchami/coresmd/internal/debug"
	"github.com/openchami/coresmd/internal/ipxe"
	"github.com/openchami/coresmd/internal/version"
)

type IfaceInfo struct {
	CompID  string
	CompNID int64
	Type    string
	MAC     string
	IPList  []net.IP
}

var log = logger.GetLogger("plugins/coresmd")

var Plugin = plugins.Plugin{
	Name:   "coresmd",
	Setup6: setup6,
	Setup4: setup4,
}

var (
	cache               *Cache
	baseURL             *url.URL
	bootScriptBaseURL   *url.URL
	leaseDuration       time.Duration
	singlePort          bool
	nodeHostnamePattern string
	bmcHostnamePattern  string
	hostnameDomain      string
)

const (
	defaultTFTPDirectory = "/tftpboot"
	defaultTFTPPort      = 69
)

func logVersion() {
	log.Infof("initializing coresmd/coresmd %s (%s), built %s", version.Version, version.GitState, version.BuildTime)
	log.WithFields(version.VersionInfo).Debugln("detailed version info")
}

func setup6(args ...string) (handler.Handler6, error) {
	return nil, errors.New("coresmd does not currently support DHCPv6")
}

func setup4(args ...string) (handler.Handler4, error) {
	logVersion()

	var err error
	var smdClient *SmdClient

	// Check if using new config file format (1 arg) or old format (6+ args)
	if len(args) == 1 {
		// NEW FORMAT: Single argument is path to config file
		log.Infof("loading configuration from file: %s", args[0])
		config, err := LoadConfig(args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}

		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}

		// Create new SmdClient
		log.Debug("generating new SmdClient")
		baseURL, err = url.Parse(config.SMD.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse base URL: %w", err)
		}
		smdClient = NewSmdClient(baseURL)

		// Parse boot script base URL
		log.Debug("parsing boot script base URL")
		bootScriptBaseURL, err = url.Parse(config.Boot.ScriptBaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse boot script base URL: %w", err)
		}

		// Set CA certificate if provided
		if config.SMD.CACertPath != "" {
			log.Infof("setting CA certificate from %s", config.SMD.CACertPath)
			if err := smdClient.UseCACert(config.SMD.CACertPath); err != nil {
				return nil, fmt.Errorf("failed to set CA certificate: %w", err)
			}
		} else {
			log.Info("CA certificate path not configured")
		}

		// Create cache
		log.Debug("generating new Cache")
		cache, err = NewCache(config.SMD.CacheDuration, smdClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create new cache: %w", err)
		}

		// Set lease duration
		log.Debug("setting lease duration")
		leaseDuration, err = time.ParseDuration(config.DHCP.LeaseDuration)
		if err != nil {
			return nil, fmt.Errorf("failed to parse lease duration: %w", err)
		}

		// Set TFTP mode
		singlePort = config.TFTP.SinglePort

		// Set hostname patterns
		nodeHostnamePattern = config.Hostname.NodePattern
		bmcHostnamePattern = config.Hostname.BMCPattern
		hostnameDomain = config.Hostname.Domain
		log.Infof("hostname config - node: %s, BMC: %s, domain: %s",
			nodeHostnamePattern, bmcHostnamePattern, hostnameDomain)

	} else if len(args) == 6 {
		// OLD FORMAT: 6 arguments (backwards compatibility)
		log.Warn("using deprecated argument format, consider migrating to config file")

		// Create new SmdClient using first argument (base URL)
		log.Debug("generating new SmdClient")
		baseURL, err = url.Parse(args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse base URL: %w", err)
		}
		smdClient = NewSmdClient(baseURL)

		// Parse from the second argument the insecure URL used by iPXE clients
		log.Debug("parsing boot script base URL")
		bootScriptBaseURL, err = url.Parse(args[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse boot script base URL: %w", err)
		}

		// If nonempty, test that CA cert path exists (third argument)
		caCertPath := strings.Trim(args[2], `"'`)
		log.Infof("cacertPath: %s", caCertPath)
		if caCertPath != "" {
			if err := smdClient.UseCACert(caCertPath); err != nil {
				return nil, fmt.Errorf("failed to set CA certificate: %w", err)
			}
			log.Infof("set CA certificate for SMD to the contents of %s", caCertPath)
		} else {
			log.Infof("CA certificate path was empty, not setting")
		}

		// Create new Cache using fourth argument (cache validity duration)
		log.Debug("generating new Cache")
		cache, err = NewCache(args[3], smdClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create new cache: %w", err)
		}

		// Set lease duration from fifth argument
		log.Debug("setting lease duration")
		leaseDuration, err = time.ParseDuration(args[4])
		if err != nil {
			return nil, fmt.Errorf("failed to parse lease duration: %w", err)
		}

		// Parse single port mode from sixth argument
		log.Debug("determining port mode")
		singlePort, err = strconv.ParseBool(args[5])
		if err != nil {
			return nil, fmt.Errorf("invalid single port toggle '%s', use 'true' or 'false'", args[5])
		}

		// Use default hostname patterns for backwards compatibility
		nodeHostnamePattern = "nid{04d}"
		bmcHostnamePattern = ""
		hostnameDomain = ""
		log.Info("using default hostname pattern: nid{04d} (no BMC pattern, no domain)")

	} else {
		return nil, errors.New("expected either 1 argument (config file path) or 6 arguments (legacy: base URL, boot script base URL, CA certificate path, cache duration, lease duration, single port mode)")
	}

	cache.RefreshLoop()

	// Start tftpserver
	log.Infof("starting TFTP server on port %d with directory %s", defaultTFTPPort, defaultTFTPDirectory)
	server := &tftpServer{
		directory:  defaultTFTPDirectory,
		port:       defaultTFTPPort,
		singlePort: singlePort,
	}

	go server.Start()

	log.Infof("coresmd plugin initialized with base URL %s and validity duration %s", smdClient.BaseURL, cache.Duration.String())

	return Handler4, nil
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	log.Debugf("HANDLER CALLED ON MESSAGE TYPE: req(%s), resp(%s)", req.MessageType(), resp.MessageType())
	debug.DebugRequest(log, req)

	// Make sure cache doesn't get updated while reading
	(*cache).Mutex.RLock()
	defer cache.Mutex.RUnlock()

	// STEP 1: Assign IP address
	hwAddr := req.ClientHWAddr.String()
	ifaceInfo, err := lookupMAC(hwAddr)
	if err != nil {
		log.Errorf("IP lookup failed: %v", err)
		return resp, false
	}
	assignedIP := ifaceInfo.IPList[0].To4()
	resp.YourIPAddr = assignedIP

	// Set lease time
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(leaseDuration))
	log.Infof("assigning %s to %s (%s) with a lease duration of %s", assignedIP, ifaceInfo.MAC, ifaceInfo.Type, leaseDuration)

	// Set client hostname
	if ifaceInfo.Type == "Node" && nodeHostnamePattern != "" {
		hostname := expandHostnamePattern(nodeHostnamePattern, ifaceInfo.CompNID, ifaceInfo.CompID)
		if hostnameDomain != "" {
			hostname = hostname + "." + hostnameDomain
		}
		resp.Options.Update(dhcpv4.OptHostName(hostname))
		log.Debugf("setting hostname to %s for node %s", hostname, ifaceInfo.CompID)
	} else if ifaceInfo.Type == "NodeBMC" && bmcHostnamePattern != "" {
		hostname := expandHostnamePattern(bmcHostnamePattern, ifaceInfo.CompNID, ifaceInfo.CompID)
		if hostnameDomain != "" {
			hostname = hostname + "." + hostnameDomain
		}
		resp.Options.Update(dhcpv4.OptHostName(hostname))
		log.Debugf("setting hostname to %s for BMC %s", hostname, ifaceInfo.CompID)
	}

	// Set root path to this server's IP
	resp.Options.Update(dhcpv4.OptRootPath(resp.ServerIPAddr.String()))

	// STEP 2: Send boot config
	if cinfo := req.Options.Get(dhcpv4.OptionUserClassInformation); string(cinfo) != "iPXE" {
		// BOOT STAGE 1: Send iPXE bootloader over TFTP
		resp, _ = ipxe.ServeIPXEBootloader(log, req, resp)
	} else {
		// BOOT STAGE 2: Send URL to BSS boot script
		bssURL := bootScriptBaseURL.JoinPath("/boot/v1/bootscript")
		bssURL.RawQuery = fmt.Sprintf("mac=%s", hwAddr)
		resp.Options.Update(dhcpv4.OptBootFileName(bssURL.String()))
	}

	debug.DebugResponse(log, resp)

	return resp, true
}

func lookupMAC(mac string) (IfaceInfo, error) {
	var ii IfaceInfo

	// Match MAC address with EthernetInterface
	ei, ok := cache.EthernetInterfaces[mac]
	if !ok {
		return ii, fmt.Errorf("no EthernetInterfaces were found in cache for hardware address %s", mac)
	}
	ii.MAC = mac

	// If found, make sure Component exists with ID matching to EthernetInterface ID
	ii.CompID = ei.ComponentID
	log.Debugf("EthernetInterface found in cache for hardware address %s with ID %s", ii.MAC, ii.CompID)
	comp, ok := cache.Components[ii.CompID]
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

// expandHostnamePattern replaces {Nd} with zero-padded NID and {id} with xname
// Example patterns:
//   - "nid{04d}" with NID=1 => "nid0001"
//   - "dev-s{02d}" with NID=5 => "dev-s05"
//   - "bmc{03d}" with NID=42 => "bmc042"
//   - "{id}" with xname="x3000c0s0b1" => "x3000c0s0b1"
func expandHostnamePattern(pattern string, nid int64, id string) string {
	out := strings.ReplaceAll(pattern, "{id}", id)
	re := regexp.MustCompile(`\{0*(\d+)d\}`)
	out = re.ReplaceAllStringFunc(out, func(m string) string {
		nStr := re.FindStringSubmatch(m)[1]
		n, _ := strconv.Atoi(nStr)
		return fmt.Sprintf("%0*d", n, nid)
	})
	return out
}
