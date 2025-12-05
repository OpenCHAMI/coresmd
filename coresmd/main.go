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

// pluginConfig holds the parsed configuration for the coresmd plugin
type pluginConfig struct {
	SmdURL        string
	BootScriptURL string
	CACertPath    string
	CacheDuration string
	LeaseDuration string
	SinglePort    bool
	NodePattern   string
	BmcPattern    string
	Domain        string
}

func logVersion() {
	log.Infof("initializing coresmd/coresmd %s (%s), built %s", version.Version, version.GitState, version.BuildTime)
	log.WithFields(version.VersionInfo).Debugln("detailed version info")
}

func setup6(args ...string) (handler.Handler6, error) {
	return nil, errors.New("coresmd does not currently support DHCPv6")
}

// parseSetup4Args parses the arguments for setup4 and returns a pluginConfig.
// It supports both key=value format and legacy positional arguments (6 or 9 args).
func parseSetup4Args(args ...string) (*pluginConfig, error) {
	config := &pluginConfig{
		NodePattern: "nid{04d}",
		BmcPattern:  "",
		Domain:      "",
		SinglePort:  false,
	}

	// Detect format: key=value pairs vs legacy positional arguments
	useKeyValue := false
	for _, arg := range args {
		if strings.Contains(arg, "=") {
			useKeyValue = true
			break
		}
	}

	if useKeyValue {
		// NEW KEY-VALUE FORMAT: Parse key=value arguments
		configMap := make(map[string]string)

		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid argument format '%s', expected key=value", arg)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			configMap[key] = value
		}

		// Validate and extract required parameters
		requiredKeys := []string{"smd_url", "boot_script_url", "cache_duration", "lease_duration"}
		for _, key := range requiredKeys {
			if _, ok := configMap[key]; !ok {
				return nil, fmt.Errorf("missing required parameter: %s", key)
			}
		}

		config.SmdURL = configMap["smd_url"]
		config.BootScriptURL = configMap["boot_script_url"]
		config.CacheDuration = configMap["cache_duration"]
		config.LeaseDuration = configMap["lease_duration"]

		// Optional parameters
		if val, ok := configMap["ca_cert"]; ok {
			config.CACertPath = val
		}
		if val, ok := configMap["node_pattern"]; ok {
			config.NodePattern = val
		}
		if val, ok := configMap["bmc_pattern"]; ok {
			config.BmcPattern = val
		}
		if val, ok := configMap["domain"]; ok {
			config.Domain = val
		}
		if val, ok := configMap["single_port"]; ok {
			singlePortBool, err := strconv.ParseBool(val)
			if err != nil {
				return nil, fmt.Errorf("invalid single_port value '%s', use 'true' or 'false'", val)
			}
			config.SinglePort = singlePortBool
		}

	} else if len(args) == 6 || len(args) == 9 {
		// LEGACY POSITIONAL FORMAT: backwards compatibility
		config.SmdURL = args[0]
		config.BootScriptURL = args[1]
		config.CACertPath = strings.Trim(args[2], `"'`)
		config.CacheDuration = args[3]
		config.LeaseDuration = args[4]

		singlePortBool, err := strconv.ParseBool(args[5])
		if err != nil {
			return nil, fmt.Errorf("invalid single port toggle '%s'", args[5])
		}
		config.SinglePort = singlePortBool

		// Handle hostname configuration (args 6-8 if present)
		if len(args) == 9 {
			config.NodePattern = strings.Trim(args[6], `"'`)
			config.BmcPattern = strings.Trim(args[7], `"'`)
			config.Domain = strings.Trim(args[8], `"'`)
		}

	} else {
		return nil, fmt.Errorf("invalid arguments: use key=value format or legacy positional format (6 or 9 args), got %d args", len(args))
	}

	return config, nil
}

func setup4(args ...string) (handler.Handler4, error) {
	logVersion()

	// Parse arguments
	config, err := parseSetup4Args(args...)
	if err != nil {
		return nil, err
	}

	// Log format being used
	for _, arg := range args {
		if strings.Contains(arg, "=") {
			log.Debug("using key=value configuration format")
			break
		}
	}
	if len(args) == 6 || len(args) == 9 {
		log.Warn("using legacy positional argument format, consider migrating to key=value format")
	}

	// Create SMD client
	log.Debug("generating new SmdClient")
	baseURL, err = url.Parse(config.SmdURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse smd_url: %w", err)
	}
	smdClient := NewSmdClient(baseURL)

	// Parse boot script URL
	log.Debug("parsing boot script base URL")
	bootScriptBaseURL, err = url.Parse(config.BootScriptURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse boot_script_url: %w", err)
	}

	// Set CA certificate if provided
	if config.CACertPath != "" {
		log.Infof("setting CA certificate from %s", config.CACertPath)
		if err := smdClient.UseCACert(config.CACertPath); err != nil {
			return nil, fmt.Errorf("failed to set CA certificate: %w", err)
		}
	} else {
		log.Info("CA certificate path not configured")
	}

	// Create cache
	log.Debug("generating new Cache")
	cache, err = NewCache(config.CacheDuration, smdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Set lease duration
	log.Debug("setting lease duration")
	leaseDuration, err = time.ParseDuration(config.LeaseDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lease_duration: %w", err)
	}

	// Set TFTP single port mode
	singlePort = config.SinglePort

	// Set hostname patterns
	nodeHostnamePattern = config.NodePattern
	bmcHostnamePattern = config.BmcPattern
	hostnameDomain = config.Domain
	log.Infof("hostname config - node: %s, BMC: %s, domain: %s",
		nodeHostnamePattern, bmcHostnamePattern, hostnameDomain)

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
