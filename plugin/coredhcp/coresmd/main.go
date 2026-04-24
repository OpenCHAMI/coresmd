// SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

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
	"github.com/sirupsen/logrus"

	"github.com/openchami/coresmd/internal/cache"
	"github.com/openchami/coresmd/internal/debug"
	"github.com/openchami/coresmd/internal/iface"
	"github.com/openchami/coresmd/internal/ipxe"
	"github.com/openchami/coresmd/internal/rule"
	"github.com/openchami/coresmd/internal/smdclient"
	"github.com/openchami/coresmd/internal/subnet"
	"github.com/openchami/coresmd/internal/tftp"
	"github.com/openchami/coresmd/internal/version"
)

type Config struct {
	// Parsed from configuration file
	svcBaseURI    *url.URL              // svc_base_uri
	ipxeBaseURI   *url.URL              // ipxe_base_uri
	caCert        string                // ca_cert
	cacheValid    *time.Duration        // cache_valid
	leaseTime     *time.Duration        // lease_time
	singlePort    bool                  // single_port
	tftpDir       string                // tftp_dir
	tftpPort      int                   // tftp_port
	domain        string                // domain
	ruleLog       string                // rule_log
	rules         []rule.Rule           // rule
	subnetContext *subnet.SubnetContext // subnet configuration
}

func (c Config) String() string {
	cfgStr := fmt.Sprintf("svc_base_uri=%s ipxe_base_uri=%s ca_cert=%s cache_valid=%s lease_time=%s single_port=%v tftp_dir=%s tftp_port=%d domain=%s rule_log=%s",
		c.svcBaseURI,
		c.ipxeBaseURI,
		c.caCert,
		c.cacheValid,
		c.leaseTime,
		c.singlePort,
		c.tftpDir,
		c.tftpPort,
		c.domain,
		c.ruleLog,
	)
	for _, rule := range c.rules {
		cfgStr += fmt.Sprintf(" rule=%s", rule)
	}

	return cfgStr
}

const (
	defaultLeaseTime   = "1h0m0s"
	defaultBMCPattern  = "bmc{04d}"
	defaultNodePattern = "nid{04d}"
)

var (
	smdCache      *cache.Cache
	globalConfig  Config
	subnetContext *subnet.SubnetContext
	log           = logger.GetLogger("plugins/coresmd")
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
	smdClient := smdclient.NewSmdClient(cfg.svcBaseURI)
	if err := smdClient.UseCACert(cfg.caCert); err != nil {
		return nil, fmt.Errorf("failed to set CA certificate: %w", err)
	}

	// Create cache and start fetching
	var err error
	if smdCache, err = cache.NewCache(log, cfg.cacheValid.String(), smdClient); err != nil {
		return nil, fmt.Errorf("failed to create new cache: %w", err)
	}
	smdCache.RefreshLoop()

	// Start tftp server
	log.Infof("starting TFTP server on port %d with directory %s", cfg.tftpPort, cfg.tftpDir)
	server := &tftp.TftpServer{
		Directory:  cfg.tftpDir,
		Port:       cfg.tftpPort,
		SinglePort: cfg.singlePort,
		Logger:     log,
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
	smdClient := smdclient.NewSmdClient(cfg.svcBaseURI)
	if err := smdClient.UseCACert(cfg.caCert); err != nil {
		return nil, fmt.Errorf("failed to set CA certificate: %w", err)
	}

	// Create cache and start fetching
	var err error
	if smdCache, err = cache.NewCache(log, cfg.cacheValid.String(), smdClient); err != nil {
		return nil, fmt.Errorf("failed to create new cache: %w", err)
	}
	smdCache.RefreshLoop()

	// Start tftp server
	log.Infof("starting TFTP server on port %d with directory %s", cfg.tftpPort, cfg.tftpDir)
	server := &tftp.TftpServer{
		Directory:  cfg.tftpDir,
		Port:       cfg.tftpPort,
		SinglePort: cfg.singlePort,
		Logger:     log,
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
	var (
		idx           int  = 0     // Separate arg index so comments aren't counted
		insideComment bool = false // Track if inside a comment
		commentIdx    int  = -1    // Track where last comment began for printing error
	)
	for argIdx, arg := range argv {
		// Parse comments (skip further arg parsing until out of comment)
		if len(arg) >= 2 {
			if arg[0:2] == "/*" || arg[len(arg)-2:] == "*/" {
				if arg[0:2] == "/*" {
					insideComment = true
					commentIdx = argIdx + 1
				}
				if arg[len(arg)-2:] == "*/" {
					if !insideComment {
						errs = append(errs, fmt.Errorf("arg %d: comment terminator (\"*/\") found without start of comment (\"/*\")", argIdx+1))
					}
					insideComment = false
				}
				continue
			}
		}
		if insideComment {
			continue
		} else {
			// If not a comment, increase separate arg index
			idx++
		}

		opt := strings.SplitN(arg, "=", 2)

		// Ensure key=val format
		if len(opt) != 2 {
			errs = append(errs, fmt.Errorf("non-comment arg %d: invalid format '%s', should be 'key=val' (skipping)", idx, arg))
			continue
		}

		// Check that key is known and, if so, process value
		switch opt[0] {
		case "svc_base_uri":
			if svcURI, err := url.Parse(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid URI '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.svcBaseURI = svcURI
			}
		case "ipxe_base_uri":
			if ipxeURI, err := url.Parse(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid URI '%s' (skipping): %w", idx, opt[0], opt[1], err))
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
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid duration '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.cacheValid = &cacheValid
			}
		case "lease_time":
			if leaseTime, err := time.ParseDuration(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid duration '%s' (skipping): %w", idx, opt[0], opt[1], err))
				continue
			} else {
				cfg.leaseTime = &leaseTime
			}
		case "single_port":
			if singlePort, err := strconv.ParseBool(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid value '%s' (defaulting to false): %w", idx, opt[0], opt[1], err))
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
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid port '%s' (defaulting to %d): %w", idx, opt[0], opt[1], tftp.DefaultTFTPPort, err))
				cfg.tftpPort = tftp.DefaultTFTPPort
			} else {
				if tftpPort >= 0 && tftpPort <= 65535 {
					cfg.tftpPort = int(tftpPort)
				} else {
					errs = append(errs, fmt.Errorf("non-comment arg %d: %s: port '%d' out of range, must be between 0-65535 (defaulting to %d)", idx, opt[0], tftpPort, tftp.DefaultTFTPPort))
					cfg.tftpPort = tftp.DefaultTFTPPort
				}
			}
		case "bmc_pattern":
			bmcPattern := strings.Trim(opt[1], `'"`)
			if bmcPattern != "" {
				bmcRuleStr := fmt.Sprintf("type:NodeBMC,hostname:%s", bmcPattern)
				if bmcRule, err := rule.ParseRule(bmcRuleStr); err != nil {
					errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid hostname rule: %q: %w", idx, opt[0], opt[1], err))
					continue
				} else {
					cfg.rules = append(cfg.rules, bmcRule)
				}
			}
		case "node_pattern":
			nodePattern := strings.Trim(opt[1], `"'`)
			if nodePattern != "" {
				nodeRuleStr := fmt.Sprintf("type:Node,hostname:%s", nodePattern)
				if nodeRule, err := rule.ParseRule(nodeRuleStr); err != nil {
					errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid hostname rule: %q: %w", idx, opt[0], opt[1], err))
					continue
				} else {
					cfg.rules = append(cfg.rules, nodeRule)
				}
			}
		case "domain":
			domain := strings.Trim(opt[1], `"'`)
			if domain != "" {
				cfg.domain = domain
			}
		case "rule_log":
			if cfg.ruleLog != "" {
				errs = append(errs, fmt.Errorf("non-comment arg %d: duplicate key '%s', using last value", idx, opt[0]))
			}
			switch opt[1] {
			case "info", "debug", "none":
				cfg.ruleLog = opt[1]
			default:
				errs = append(errs, fmt.Errorf("non-comment arg %d: invalid format for key '%s': expected 'info', 'debug', or 'none', got %s", idx, opt[0], opt[1]))
				continue
			}
		case "rule":
			rule, err := rule.ParseRule(opt[1])
			if err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid rule: %q: %w", idx, opt[0], opt[1], err))
				continue
			}
			cfg.rules = append(cfg.rules, rule)
		case "subnet":
			if cfg.subnetContext == nil {
				cfg.subnetContext = subnet.NewSubnetContext()
			}
			parts := strings.Split(opt[1], ",")
			if len(parts) != 2 {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid format '%s', expected 'cidr,router' (skipping)", idx, opt[0], opt[1]))
				continue
			}
			cidr := strings.TrimSpace(parts[0])
			router := strings.TrimSpace(parts[1])
			if err := cfg.subnetContext.AddSubnet(cidr, router); err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: failed to add subnet: %w", idx, opt[0], err))
				continue
			}
		default:
			errs = append(errs, fmt.Errorf("non-comment arg %d: unknown config key '%s' (skipping)", idx, opt[0]))
			continue
		}
	}
	if insideComment {
		errs = append(errs, fmt.Errorf("arg %d: unterminated comment (\"/*\" found without a \"*/\")", commentIdx))
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
		warns = append(warns, fmt.Sprintf("cache_valid unset, defaulting to %s", cache.DefaultCacheValid))
		duration, err := time.ParseDuration(cache.DefaultCacheValid)
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
		warns = append(warns, fmt.Sprintf("tftp_port %d out of 0-65535 range, defaulting to %d", c.tftpPort, tftp.DefaultTFTPPort))
		c.tftpPort = tftp.DefaultTFTPPort
	} else if c.tftpPort == 0 {
		warns = append(warns, fmt.Sprintf("tftp_port unset (0), defaulting to %d", tftp.DefaultTFTPPort))
		c.tftpPort = tftp.DefaultTFTPPort
	}
	if c.tftpDir == "" {
		warns = append(warns, fmt.Sprintf("tftp_dir unset, defaulting to %s", tftp.DefaultTFTPDirectory))
		c.tftpDir = tftp.DefaultTFTPDirectory
	}
	if c.domain == "" {
		warns = append(warns, "domain unset, not configuring")
	}
	if strings.TrimSpace(c.ruleLog) == "" {
		warns = append(warns, "rule_log unset, defaulting to info")
		c.ruleLog = "info"
	}
	// Apply default per-rule log behavior; if rule-level log is omitted, inherit
	// the global rule_log value.
	for i := range c.rules {
		if strings.TrimSpace(c.rules[i].Log) == "" {
			c.rules[i].Log = c.ruleLog
		}
	}
	return
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	log.Debugf("HANDLER CALLED ON MESSAGE TYPE: req(%s), resp(%s)", req.MessageType(), resp.MessageType())
	debug.DebugRequest(log, req)

	// Make sure cache doesn't get updated while reading
	(*smdCache).Mutex.RLock()
	defer smdCache.Mutex.RUnlock()

	// STEP 1: Assign IP address and set standard DHCP options
	hwAddr := req.ClientHWAddr.String()
	giaddr := req.GatewayIPAddr

	// Use subnet-aware lookup if subnet context is configured
	var ifaceInfo iface.IfaceInfo
	var err error
	if globalConfig.subnetContext != nil && !globalConfig.subnetContext.IsEmpty() {
		ifaceInfo, err = iface.LookupMACWithSubnet(log, hwAddr, giaddr, smdCache, globalConfig.subnetContext)
		if err != nil {
			log.Errorf("subnet-aware IP lookup failed for MAC %s (giaddr=%s): %v", hwAddr, giaddr, err)
			return resp, false
		}
	} else {
		ifaceInfo, err = iface.LookupMAC(log, hwAddr, smdCache)
		if err != nil {
			log.Errorf("IP lookup failed: %v", err)
			return resp, false
		}
	}

	assignedIP := ifaceInfo.IPList[0].To4()
	resp.YourIPAddr = assignedIP

	// Set lease time
	if globalConfig.leaseTime == nil {
		log.Errorf("lease time unset in global config! unable to set lease time in DHCPv4 response to %s", ifaceInfo.MAC)
	} else {
		resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(*globalConfig.leaseTime))
	}

	// Set subnet-specific router and netmask if config-level subnet= is set.
	//
	// NOTE: These DHCP options may be overridden by rule-level actions below
	// (e.g. rule=subnet:...,routers:...,netmask:...). Config-level subnet=
	// provides baseline values; rule-level actions take precedence because
	// Evaluate4 runs after this block and calls resp.Options.Update().
	if globalConfig.subnetContext != nil && !globalConfig.subnetContext.IsEmpty() {
		subnetConfig, cidr, err := globalConfig.subnetContext.FindSubnetForIP(assignedIP)
		if err == nil {
			log.Debugf("setting router %s for subnet %s", subnetConfig.Router, cidr)
			resp.Options.Update(dhcpv4.OptRouter(subnetConfig.Router))

			// Set subnet mask based on CIDR
			resp.Options.Update(dhcpv4.OptSubnetMask(subnetConfig.CIDR.Mask))
		} else {
			log.Warnf("assigned IP %s not in any configured subnet: %v", assignedIP, err)
		}
	}

	// Apply rules
	rule.Evaluate4(log, ifaceInfo, globalConfig.domain, globalConfig.ruleLog, resp, globalConfig.rules)

	// Set root path to this server's IP
	resp.Options.Update(dhcpv4.OptRootPath(resp.ServerIPAddr.String()))

	// Log assignment
	log.WithFields(logrus.Fields{
		"comp_id":           ifaceInfo.CompID,
		"comp_nid":          ifaceInfo.CompNID,
		"comp_type":         ifaceInfo.Type,
		"comp_ips":          ifaceInfo.IPList,
		"comp_mac":          ifaceInfo.MAC,
		"assigned_ipv4":     assignedIP,
		"assigned_hostname": string(resp.Options.Get(dhcpv4.OptionHostName)),
		"lease_duration":    globalConfig.leaseTime,
		"server_ip":         resp.ServerIPAddr,
		"giaddr":            giaddr,
		"router_ips":        fmt.Sprintf("%v", dhcpv4.GetIPs(dhcpv4.OptionRouter, resp.Options)),
		"netmask":           fmt.Sprintf("%v", dhcpv4.GetIP(dhcpv4.OptionSubnetMask, resp.Options)),
	}).Info("DHCPv4 assignment")

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

func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	log.Debugf("DHCPv6 HANDLER CALLED ON MESSAGE TYPE: req(%s), resp(%s)", req.Type(), resp.Type())

	// Make sure cache doesn't get updated while reading
	(*smdCache).Mutex.RLock()
	defer smdCache.Mutex.RUnlock()

	// Extract MAC address from DHCPv6 message
	hwAddr, err := dhcpv6.ExtractMAC(req)
	if err != nil {
		log.Errorf("Failed to extract MAC address from DHCPv6 request: %v", err)
		return resp, false
	}

	// STEP 1: Lookup interface info and assign IPv6 address
	macStr := hwAddr.String()
	ifaceInfo, err := iface.LookupMAC(log, macStr, smdCache)
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

	// Apply rules
	rule.Evaluate6(log, ifaceInfo, globalConfig.domain, globalConfig.ruleLog, msg, globalConfig.rules)

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
	log.WithFields(logrus.Fields{
		"comp_id":           ifaceInfo.CompID,
		"comp_nid":          ifaceInfo.CompNID,
		"comp_type":         ifaceInfo.Type,
		"comp_ips":          ifaceInfo.IPList,
		"comp_mac":          ifaceInfo.MAC,
		"assigned_ipv6":     assignedIPv6,
		"assigned_hostname": resp.GetOption(dhcpv6.OptionFQDN),
		"lease_duration":    globalConfig.leaseTime,
	}).Info("DHCPv6 assignment")

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
