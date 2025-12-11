package coresmd

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/rfc1035label"

	"github.com/openchami/coresmd/internal/debug"
	"github.com/openchami/coresmd/internal/hostname"
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

type Config struct {
	// Parsed from configuration file
	svcBaseURI  *url.URL       // svc_base_uri
	ipxeBaseURI *url.URL       // ipxe_base_uri
	caCert      string         // ca_cert
	cacheValid  *time.Duration // cache_valid
	leaseTime   *time.Duration // lease_time
	singlePort  bool           // single_port
	tftpDir     string         // tftp_dir
	tftpPort    int            // tftp_port
	bmcPattern  string         // bmc_pattern
	nodePattern string         // node_pattern
	domain      string         // domain
}

func (c Config) String() string {
	return fmt.Sprintf("svc_base_uri=%s ipxe_base_uri=%s ca_cert=%s cache_valid=%s lease_time=%s single_port=%v tftp_dir=%s tftp_port=%d bmc_pattern=%s node_pattern=%s domain=%s",
		c.svcBaseURI,
		c.ipxeBaseURI,
		c.caCert,
		c.cacheValid,
		c.leaseTime,
		c.singlePort,
		c.tftpDir,
		c.tftpPort,
		c.bmcPattern,
		c.nodePattern,
		c.domain,
	)
}

const (
	defaultTFTPDirectory = "/tftpboot"
	defaultTFTPPort      = 69
	defaultCacheValid    = "30s"
	defaultLeaseTime     = "1h0m0s"
	defaultBMCPattern    = "bmc{04d}"
	defaultNodePattern   = "nid{04d}"
)

var (
	cache        *Cache
	globalConfig Config
	log          = logger.GetLogger("plugins/coresmd")
)

var Plugin = plugins.Plugin{
	Name:   "coresmd",
	Setup6: setup6,
	Setup4: setup4,
}

func logVersion() {
	log.Infof("initializing coresmd/coresmd %s (%s), built %s", version.Version, version.GitState, version.BuildTime)
	log.WithFields(version.VersionInfo).Debugln("detailed version info")
}

func setup6(args ...string) (handler.Handler6, error) {
	logVersion()

	// Parse config from config file
	cfg, errs := parseConfig(args...)
	for _, err := range errs {
		log.Error(err)
	}

	// Validate parsed config
	warns, errs := cfg.validate()
	for _, warning := range warns {
		log.Warn(warning)
	}
	if len(errs) > 0 {
		for _, err := range errs {
			log.Error(err)
		}
		return nil, fmt.Errorf("%d fatal errors occurred, exiting", len(errs))
	}

	// Set parsed config as global to be accessed by other functions
	globalConfig = cfg

	// Create client to talk to SMD and set validating CA cert
	smdClient := NewSmdClient(cfg.svcBaseURI)
	if err := smdClient.UseCACert(cfg.caCert); err != nil {
		return nil, fmt.Errorf("failed to set CA certificate: %w", err)
	}

	// Create cache and start fetching
	var err error
	if cache, err = NewCache(cfg.cacheValid.String(), smdClient); err != nil {
		return nil, fmt.Errorf("failed to create new cache: %w", err)
	}
	cache.RefreshLoop()

	// Start tftp server
	log.Infof("starting TFTP server on port %d with directory %s", cfg.tftpPort, cfg.tftpDir)
	server := &tftpServer{
		directory:  cfg.tftpDir,
		port:       cfg.tftpPort,
		singlePort: cfg.singlePort,
	}

	go server.Start()

	log.Infof("coresmd plugin initialized with %s", cfg)

	return Handler6, nil
}

func setup4(args ...string) (handler.Handler4, error) {
	logVersion()

	// Parse config from config file
	cfg, errs := parseConfig(args...)
	for _, err := range errs {
		log.Error(err)
	}

	// Validate parsed config
	warns, errs := cfg.validate()
	for _, warning := range warns {
		log.Warn(warning)
	}
	if len(errs) > 0 {
		for _, err := range errs {
			log.Error(err)
		}
		return nil, fmt.Errorf("%d fatal errors occurred, exiting", len(errs))
	}

	// Set parsed config as global to be accessed by other functions
	globalConfig = cfg

	// Create client to talk to SMD and set validating CA cert
	smdClient := NewSmdClient(cfg.svcBaseURI)
	if err := smdClient.UseCACert(cfg.caCert); err != nil {
		return nil, fmt.Errorf("failed to set CA certificate: %w", err)
	}

	// Create cache and start fetching
	var err error
	if cache, err = NewCache(cfg.cacheValid.String(), smdClient); err != nil {
		return nil, fmt.Errorf("failed to create new cache: %w", err)
	}
	cache.RefreshLoop()

	// Start tftp server
	log.Infof("starting TFTP server on port %d with directory %s", cfg.tftpPort, cfg.tftpDir)
	server := &tftpServer{
		directory:  cfg.tftpDir,
		port:       cfg.tftpPort,
		singlePort: cfg.singlePort,
	}

	go server.Start()

	log.Infof("coresmd plugin initialized with %s", cfg)

	return Handler4, nil
}

// parseConfig takes a variadic array of string arguments representing an array
// of key=value pairs and parses them into a Config struct, returning it. If any
// errors occur, they are gathered into errs, a slice of errors, so that they
// can be printed or handled.
func parseConfig(argv ...string) (cfg Config, errs []error) {
	for idx, arg := range argv {
		opt := strings.SplitN(arg, "=", 2)

		// Ensure key=val format
		if len(opt) != 2 {
			errs = append(errs, fmt.Errorf("arg %d: invalid format '%s', should be 'key=val' (skipping)", idx, arg))
			continue
		}

		// Check that key is known and, if so, process value
		switch opt[0] {
		case "svc_base_uri":
			if svcURI, err := url.Parse(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("arg %d: %s: invalid URI '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.svcBaseURI = svcURI
			}
		case "ipxe_base_uri":
			if ipxeURI, err := url.Parse(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("arg %d: %s: invalid URI '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.ipxeBaseURI = ipxeURI
			}
		case "ca_cert":
			// Simply set if nonempty when trimmed. Checking happens later.
			caCertPath := strings.Trim(opt[1], `"'`)
			if caCertPath != "" {
				cfg.caCert = caCertPath
			}
		case "cache_valid":
			if cacheValid, err := time.ParseDuration(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("arg %d: %s: invalid duration '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.cacheValid = &cacheValid
			}
		case "lease_time":
			if leaseTime, err := time.ParseDuration(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("arg %d: %s: invalid duration '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.leaseTime = &leaseTime
			}
		case "single_port":
			if singlePort, err := strconv.ParseBool(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("arg %d: %s: invalid value '%s' (defaulting to false): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.singlePort = singlePort
			}
		case "tftp_dir":
			tftpDir := strings.Trim(opt[1], `'"`)
			if tftpDir != "" {
				cfg.tftpDir = tftpDir
			}
		case "tftp_port":
			if tftpPort, err := strconv.ParseInt(opt[1], 10, 64); err != nil {
				errs = append(errs, fmt.Errorf("arg %d: %s: invalid port '%s' (defaulting to %d): %w", idx, opt[0], opt[1], defaultTFTPPort, err))
				cfg.tftpPort = defaultTFTPPort
			} else {
				if tftpPort >= 0 && tftpPort <= 65535 {
					cfg.tftpPort = int(tftpPort)
				} else {
					errs = append(errs, fmt.Errorf("arg %d: %s: port '%d' out of range, must be between 0-65535 (defaulting to %d)", idx, opt[0], tftpPort, defaultTFTPPort))
					cfg.tftpPort = defaultTFTPPort
				}
			}
		case "bmc_pattern":
			bmcPattern := strings.Trim(opt[1], `'"`)
			if bmcPattern != "" {
				cfg.bmcPattern = bmcPattern
			}
		case "node_pattern":
			nodePattern := strings.Trim(opt[1], `"'`)
			if nodePattern != "" {
				cfg.nodePattern = nodePattern
			}
		case "domain":
			domain := strings.Trim(opt[1], `"'`)
			if domain != "" {
				cfg.domain = domain
			}
		default:
			errs = append(errs, fmt.Errorf("arg %d: unknown config key '%s' (skipping)", idx, opt[0]))
			continue
		}
	}
	return
}

// validate validates a Config, putting warnings in warns (a []string) and fatal
// errors in errs (a []error) so that they can be printed and handled. For
// members of Config that support default values, default values will be set for
// them if invalid values are detected.
func (c *Config) validate() (warns []string, errs []error) {
	if c.svcBaseURI == nil {
		errs = append(errs, fmt.Errorf("svc_base_uri is required"))
	}
	if c.ipxeBaseURI == nil {
		errs = append(errs, fmt.Errorf("ipxe_base_uri is required"))
	}
	if c.caCert == "" {
		warns = append(warns, "ca_cert unset, TLS certificates will not be validated")
	}
	if c.cacheValid == nil {
		warns = append(warns, fmt.Sprintf("cache_valid unset, defaulting to %s", defaultCacheValid))
		duration, err := time.ParseDuration(defaultCacheValid)
		if err != nil {
			errs = append(errs, fmt.Errorf("unexpected error trying to set default cache_valid: %w", err))
		} else {
			c.cacheValid = &duration
		}
	}
	if c.leaseTime == nil {
		warns = append(warns, fmt.Sprintf("lease_time unset, defaulting to %s", defaultLeaseTime))
		duration, err := time.ParseDuration(defaultLeaseTime)
		if err != nil {
			errs = append(errs, fmt.Errorf("unexpected error trying to set default lease_time: %w", err))
		} else {
			c.leaseTime = &duration
		}
	}
	if c.tftpPort < 0 || c.tftpPort > 65535 {
		warns = append(warns, fmt.Sprintf("tftp_port %d out of 0-65535 range, defaulting to %d", c.tftpPort, defaultTFTPPort))
		c.tftpPort = defaultTFTPPort
	} else if c.tftpPort == 0 {
		warns = append(warns, fmt.Sprintf("tftp_port unset (0), defaulting to %d", defaultTFTPPort))
		c.tftpPort = defaultTFTPPort
	}
	if c.tftpDir == "" {
		warns = append(warns, fmt.Sprintf("tftp_dir unset, defaulting to %s", defaultTFTPDirectory))
		c.tftpDir = defaultTFTPDirectory
	}
	if c.bmcPattern == "" {
		warns = append(warns, fmt.Sprintf("bmc_pattern unset, defaulting to %s", defaultBMCPattern))
		c.bmcPattern = defaultBMCPattern
	}
	if c.nodePattern == "" {
		warns = append(warns, fmt.Sprintf("node_pattern unset, defaulting to %s", defaultNodePattern))
		c.nodePattern = defaultNodePattern
	}
	if c.domain == "" {
		warns = append(warns, "domain unset, not configuring")
	}
	return
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
	if globalConfig.leaseTime == nil {
		log.Errorf("lease time unset in global config! unable to set lease time in DHCPv4 response to %s", ifaceInfo.MAC)
	} else {
		resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(*globalConfig.leaseTime))
	}

	// Set client hostname
	hname := "(none)"
	if ifaceInfo.Type == "Node" {
		nodeHostname := hostname.ExpandHostnamePattern(globalConfig.nodePattern, ifaceInfo.CompNID, ifaceInfo.CompID)
		if globalConfig.domain != "" {
			nodeHostname = nodeHostname + "." + globalConfig.domain
		}
		hname = nodeHostname
		resp.Options.Update(dhcpv4.OptHostName(nodeHostname))
		log.Debugf("setting hostname for node %s to %s", ifaceInfo.CompID, nodeHostname)
	} else if ifaceInfo.Type == "NodeBMC" {
		bmcHostname := hostname.ExpandHostnamePattern(globalConfig.bmcPattern, ifaceInfo.CompNID, ifaceInfo.CompID)
		if globalConfig.domain != "" {
			bmcHostname = bmcHostname + "." + globalConfig.domain
		}
		hname = bmcHostname
		resp.Options.Update(dhcpv4.OptHostName(bmcHostname))
		log.Debugf("setting hostname for BMC %s to %s", ifaceInfo.CompID, bmcHostname)
	}

	// Log assignment
	log.Infof("assigning IP %s and hostname %s to %s (%s) with a lease duration of %s", assignedIP, hname, ifaceInfo.MAC, ifaceInfo.Type, globalConfig.leaseTime)

	// Set root path to this server's IP
	resp.Options.Update(dhcpv4.OptRootPath(resp.ServerIPAddr.String()))

	// STEP 2: Send boot config
	if cinfo := req.Options.Get(dhcpv4.OptionUserClassInformation); string(cinfo) != "iPXE" {
		// BOOT STAGE 1: Send iPXE bootloader over TFTP
		resp, _ = ipxe.ServeIPXEBootloader(log, req, resp)
	} else {
		// BOOT STAGE 2: Send URL to BSS boot script
		bssURL := globalConfig.ipxeBaseURI.JoinPath("/boot/v1/bootscript")
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

func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	log.Debugf("DHCPv6 HANDLER CALLED ON MESSAGE TYPE: req(%s), resp(%s)", req.Type(), resp.Type())

	// Make sure cache doesn't get updated while reading
	(*cache).Mutex.RLock()
	defer cache.Mutex.RUnlock()

	// Extract MAC address from DHCPv6 message
	hwAddr, err := dhcpv6.ExtractMAC(req)
	if err != nil {
		log.Errorf("Failed to extract MAC address from DHCPv6 request: %v", err)
		return resp, false
	}

	// STEP 1: Lookup interface info and assign IPv6 address
	macStr := hwAddr.String()
	ifaceInfo, err := lookupMAC(macStr)
	if err != nil {
		log.Errorf("IP lookup failed: %v", err)
		return resp, false
	}

	// Find an IPv6 address from the IP list
	var assignedIPv6 net.IP
	for _, ip := range ifaceInfo.IPList {
		if ip.To4() == nil && ip.To16() != nil {
			// This is an IPv6 address
			assignedIPv6 = ip
			break
		}
	}

	if assignedIPv6 == nil {
		log.Errorf("No IPv6 address found for MAC %s", macStr)
		return resp, false
	}

	// Get the message and modify it
	msg, ok := resp.(*dhcpv6.Message)
	if !ok {
		log.Errorf("Response is not a DHCPv6 message")
		return resp, false
	}

	// Set client hostname
	hname := "(none)"
	if ifaceInfo.Type == "Node" {
		nodeHostname := hostname.ExpandHostnamePattern(globalConfig.nodePattern, ifaceInfo.CompNID, ifaceInfo.CompID)
		if globalConfig.domain != "" {
			nodeHostname = nodeHostname + "." + globalConfig.domain
		}
		hname = nodeHostname
		labels := &rfc1035label.Labels{Labels: strings.Split(nodeHostname, ".")}
		msg.UpdateOption(&dhcpv6.OptFQDN{Flags: 0, DomainName: labels})
		log.Debugf("setting hostname for node %s to %s", ifaceInfo.CompID, nodeHostname)
	} else if ifaceInfo.Type == "NodeBMC" {
		bmcHostname := hostname.ExpandHostnamePattern(globalConfig.bmcPattern, ifaceInfo.CompNID, ifaceInfo.CompID)
		if globalConfig.domain != "" {
			bmcHostname = bmcHostname + "." + globalConfig.domain
		}
		hname = bmcHostname
		labels := &rfc1035label.Labels{Labels: strings.Split(bmcHostname, ".")}
		msg.UpdateOption(&dhcpv6.OptFQDN{Flags: 0, DomainName: labels})
		log.Debugf("setting hostname for BMC %s to %s", ifaceInfo.CompID, bmcHostname)
	}

	// Add IANA (Identity Association for Non-temporary Addresses) with the IPv6 address
	reqMsg, ok := req.(*dhcpv6.Message)
	if ok {
		// Get IAID from request if present
		if iana := reqMsg.Options.OneIANA(); iana != nil {
			// Create IANA with the assigned address
			ianaOpt := &dhcpv6.OptIANA{
				IaId: iana.IaId,
				T1:   time.Duration(globalConfig.leaseTime.Seconds()/2) * time.Second,
				T2:   time.Duration(globalConfig.leaseTime.Seconds()*3/4) * time.Second,
				Options: dhcpv6.IdentityOptions{
					Options: []dhcpv6.Option{
						&dhcpv6.OptIAAddress{
							IPv6Addr:          assignedIPv6,
							PreferredLifetime: *globalConfig.leaseTime,
							ValidLifetime:     *globalConfig.leaseTime,
						},
					},
				},
			}
			msg.UpdateOption(ianaOpt)
		}
	}

	// Log assignment
	log.Infof("assigning IPv6 %s and hostname %s to %s (%s) with a lease duration of %s", assignedIPv6, hname, ifaceInfo.MAC, ifaceInfo.Type, globalConfig.leaseTime)

	// STEP 2: Send boot config for iPXE
	if reqMsg, ok := req.(*dhcpv6.Message); ok {
		if uc := reqMsg.GetOneOption(dhcpv6.OptionUserClass); uc != nil {
			if userClass, ok := uc.(*dhcpv6.OptUserClass); ok {
				// Check if this is iPXE
				isPXE := false
				for _, data := range userClass.UserClasses {
					if string(data) == "iPXE" {
						isPXE = true
						break
					}
				}

				if isPXE {
					// BOOT STAGE 2: Send URL to BSS boot script
					bssURL := globalConfig.ipxeBaseURI.JoinPath("/boot/v1/bootscript")
					bssURL.RawQuery = fmt.Sprintf("mac=%s", macStr)
					msg.UpdateOption(dhcpv6.OptBootFileURL(bssURL.String()))
				} else {
					// BOOT STAGE 1: Send iPXE bootloader URL
					// For DHCPv6, we need to provide the bootfile URL
					// Get server ID from response message
					if serverID := msg.GetOneOption(dhcpv6.OptionServerID); serverID != nil {
						tftpURL := fmt.Sprintf("tftp://[%s]:%d/ipxe.efi", serverID, globalConfig.tftpPort)
						msg.UpdateOption(dhcpv6.OptBootFileURL(tftpURL))
					}
				}
			}
		}
	}

	return msg, true
}
