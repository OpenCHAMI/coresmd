// SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package bootloop

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/coredhcp/coredhcp/plugins/allocators"
	"github.com/coredhcp/coredhcp/plugins/allocators/bitmap"
	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/openchami/coresmd/internal/debug"
	"github.com/openchami/coresmd/internal/ipxe"
	"github.com/openchami/coresmd/internal/subnet"
	"github.com/openchami/coresmd/internal/version"
)

// Record holds an IP lease record
type Record struct {
	IP       net.IP
	expires  int
	hostname string
}

// PluginState is the data held by an instance of the bootloop plugin
type PluginState struct {
	// Rough lock for the whole plugin
	sync.Mutex
	// Recordsv4 holds a MAC -> IP address and lease time mapping
	Recordsv4      map[string]*Record
	LeaseTime      time.Duration
	leasedb        *sql.DB
	allocator      allocators.Allocator
	subnetPoolMgr  *subnet.SubnetPoolManager
	useSubnetPools bool
}

type Config struct {
	// Parsed from configuration file
	leaseFile  string         // lease_file
	leaseTime  *time.Duration // lease_time
	ipv4Start  *net.IP        // ipv4_start (legacy single pool)
	ipv4End    *net.IP        // ipv4_end (legacy single pool)
	scriptPath string         // script_path

	// Subnet-aware pools
	subnetPools map[string]*SubnetPoolConfig // subnet_pool configurations

	// Used, but not parse from configuration
	ipv4Range uint32 // ipv4_range
}

// SubnetPoolConfig represents a pool configuration for a specific subnet
type SubnetPoolConfig struct {
	CIDR     string
	StartIP  net.IP
	EndIP    net.IP
	IPv4Range uint32
}

func (c Config) String() string {
	return fmt.Sprintf("ipv4_start=%s ipv4_end=%s ipv4_range=%d script_path=%s",
		c.ipv4Start,
		c.ipv4End,
		c.ipv4Range,
		c.scriptPath,
	)
}

const (
	defaultLeaseTime  = "5m"
	defaultScriptPath = "default"
)

var (
	globalConfig Config
	log          = logger.GetLogger("plugins/bootloop")
	p            PluginState
)

var Plugin = plugins.Plugin{
	Name:   "bootloop",
	Setup6: setup6,
	Setup4: setup4,
}

func logVersion() {
	log.Infof("initializing coresmd/bootloop %s (%s), built %s", version.Version, version.GitState, version.BuildTime)
	log.WithFields(version.VersionInfo).Debugln("detailed version info")
}

func setup6(args ...string) (handler.Handler6, error) {
	logVersion()
	return nil, errors.New("bootloop does not currently support DHCPv6")
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
	p.LeaseTime = *cfg.leaseTime

	// Check if using subnet pools or legacy single pool
	var err error
	if len(cfg.subnetPools) > 0 {
		// Use subnet-aware pools
		log.Infof("configuring %d subnet-specific IP pools", len(cfg.subnetPools))
		p.subnetPoolMgr = subnet.NewSubnetPoolManager()
		p.useSubnetPools = true

		for cidr, poolCfg := range cfg.subnetPools {
			log.Infof("adding pool for subnet %s: %s - %s", cidr, poolCfg.StartIP, poolCfg.EndIP)
			if err := p.subnetPoolMgr.AddPool(cidr, poolCfg.StartIP, poolCfg.EndIP); err != nil {
				return nil, fmt.Errorf("failed to add pool for subnet %s: %w", cidr, err)
			}
		}
	} else {
		// Use legacy single pool
		log.Info("using legacy single IP pool")
		p.useSubnetPools = false
		p.allocator, err = bitmap.NewIPv4Allocator(*cfg.ipv4Start, *cfg.ipv4End)
		if err != nil {
			return nil, fmt.Errorf("failed to create an allocator: %w", err)
		}
	}

	// Set up storage backend using passed file path
	if err := p.registerBackingDB(cfg.leaseFile); err != nil {
		return nil, fmt.Errorf("failed to setup lease storage: %w", err)
	}
	p.Recordsv4, err = loadRecords(p.leasedb)
	if err != nil {
		return nil, fmt.Errorf("failed to load records from file: %w", err)
	}

	// Allocate any pre-existing leases
	if p.useSubnetPools {
		// For subnet pools, we need to determine which pool each IP belongs to
		for mac, v := range p.Recordsv4 {
			allocated := false
			for cidr, pool := range p.subnetPoolMgr.Pools {
				if pool.CIDR.Contains(v.IP) {
					allocator, err := p.subnetPoolMgr.GetAllocatorForSubnet(cidr)
					if err != nil {
						return nil, fmt.Errorf("failed to get allocator for subnet %s: %w", cidr, err)
					}
					ip, err := allocator.Allocate(net.IPNet{IP: v.IP})
					if err != nil {
						return nil, fmt.Errorf("failed to re-allocate leased ip %v in subnet %s: %v", v.IP.String(), cidr, err)
					}
					if ip.IP.String() != v.IP.String() {
						return nil, fmt.Errorf("allocator did not re-allocate requested leased ip %v: %v", v.IP.String(), ip.String())
					}
					allocated = true
					log.Debugf("re-allocated IP %s for MAC %s in subnet %s", v.IP, mac, cidr)
					break
				}
			}
			if !allocated {
				log.Warnf("existing lease for MAC %s with IP %s does not match any configured subnet", mac, v.IP)
			}
		}
	} else {
		// Legacy single pool
		for _, v := range p.Recordsv4 {
			ip, err := p.allocator.Allocate(net.IPNet{IP: v.IP})
			if err != nil {
				return nil, fmt.Errorf("failed to re-allocate leased ip %v: %v", v.IP.String(), err)
			}
			if ip.IP.String() != v.IP.String() {
				return nil, fmt.Errorf("allocator did not re-allocate requested leased ip %v: %v", v.IP.String(), ip.String())
			}
		}
	}

	log.Infof("bootloop plugin initialized with %s", cfg)

	return p.Handler4, nil
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
		case "lease_file":
			leaseFile := strings.Trim(opt[1], `"'`)
			if leaseFile == "" {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: empty (skipping)", idx, opt[0]))
				continue
			} else {
				cfg.leaseFile = leaseFile
			}
		case "script_path":
			scriptPath := strings.Trim(opt[1], `"'`)
			if scriptPath == "" {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: empty (setting to default script)", idx, opt[0]))
				cfg.scriptPath = defaultScriptPath
				continue
			} else {
				cfg.scriptPath = scriptPath
			}
		case "lease_time":
			if leaseTime, err := time.ParseDuration(opt[1]); err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid duration '%s' (skipping)", idx, opt[0], opt[1]))
				continue
			} else {
				cfg.leaseTime = &leaseTime
			}
		case "ipv4_start":
			ipv4Start := net.ParseIP(opt[1])
			if ipv4Start.To4() == nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid ip address '%s' (skipping)", idx, opt[0], opt[1]))
				continue
			} else {
				cfg.ipv4Start = &ipv4Start
			}
		case "ipv4_end":
			ipv4End := net.ParseIP(opt[1])
			if ipv4End.To4() == nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid ip address '%s' (skipping)", idx, opt[0], opt[1]))
				continue
			} else {
				cfg.ipv4End = &ipv4End
			}
		case "subnet_pool":
			if cfg.subnetPools == nil {
				cfg.subnetPools = make(map[string]*SubnetPoolConfig)
			}
			parts := strings.Split(opt[1], ",")
			if len(parts) != 3 {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid format '%s', expected 'cidr,start_ip,end_ip' (skipping)", idx, opt[0], opt[1]))
				continue
			}
			cidr := strings.TrimSpace(parts[0])
			startIPStr := strings.TrimSpace(parts[1])
			endIPStr := strings.TrimSpace(parts[2])

			// Validate CIDR
			_, ipnet, err := net.ParseCIDR(cidr)
			if err != nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid CIDR '%s': %w", idx, opt[0], cidr, err))
				continue
			}

			// Parse and validate start IP
			startIP := net.ParseIP(startIPStr)
			if startIP == nil || startIP.To4() == nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid start IP '%s' (skipping)", idx, opt[0], startIPStr))
				continue
			}
			if !ipnet.Contains(startIP) {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: start IP '%s' not in subnet '%s' (skipping)", idx, opt[0], startIPStr, cidr))
				continue
			}

			// Parse and validate end IP
			endIP := net.ParseIP(endIPStr)
			if endIP == nil || endIP.To4() == nil {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: invalid end IP '%s' (skipping)", idx, opt[0], endIPStr))
				continue
			}
			if !ipnet.Contains(endIP) {
				errs = append(errs, fmt.Errorf("non-comment arg %d: %s: end IP '%s' not in subnet '%s' (skipping)", idx, opt[0], endIPStr, cidr))
				continue
			}

			// Calculate range
			ipv4Range := binary.BigEndian.Uint32(endIP.To4()) - binary.BigEndian.Uint32(startIP.To4()) + 1

			cfg.subnetPools[cidr] = &SubnetPoolConfig{
				CIDR:     cidr,
				StartIP:  startIP,
				EndIP:    endIP,
				IPv4Range: ipv4Range,
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
	if c.leaseFile == "" {
		errs = append(errs, fmt.Errorf("lease_file is required"))
	}

	// Check if using subnet pools or legacy configuration
	usingSubnetPools := len(c.subnetPools) > 0
	usingLegacyPool := c.ipv4Start != nil || c.ipv4End != nil

	if usingSubnetPools && usingLegacyPool {
		errs = append(errs, fmt.Errorf("cannot use both subnet_pool and legacy ipv4_start/ipv4_end configuration"))
	} else if !usingSubnetPools && !usingLegacyPool {
		errs = append(errs, fmt.Errorf("must configure either subnet_pool or ipv4_start/ipv4_end"))
	}

	// Validate legacy pool configuration
	if usingLegacyPool && !usingSubnetPools {
		if c.ipv4Start == nil {
			errs = append(errs, fmt.Errorf("ipv4_start is required when not using subnet_pool"))
		}
		if c.ipv4End == nil {
			errs = append(errs, fmt.Errorf("ipv4_end is required when not using subnet_pool"))
		}
		if c.ipv4Start != nil && c.ipv4End != nil {
			// Ensure IP range is valid
			if binary.BigEndian.Uint32(c.ipv4Start.To4()) > binary.BigEndian.Uint32(c.ipv4End.To4()) {
				errs = append(errs, fmt.Errorf("invalid range: ipv4_end (%s) must be equal to or higher than ipv4_start (%s)", c.ipv4End.To4(), c.ipv4Start.To4()))
			} else {
				// Calculate number of IP addresses in range
				c.ipv4Range = binary.BigEndian.Uint32(c.ipv4End.To4()) - binary.BigEndian.Uint32(c.ipv4Start.To4()) + 1
			}
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
	if c.scriptPath == "" {
		warns = append(warns, fmt.Sprintf("script_path unset, using default"))
		c.scriptPath = defaultScriptPath
	}
	return
}

func (p *PluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	// Make sure db doesn't get updated while reading
	p.Lock()
	defer p.Unlock()

	debug.DebugRequest(log, req)

	// Set root path to this server's IP
	resp.Options.Update(dhcpv4.OptRootPath(resp.ServerIPAddr.String()))

	giaddr := req.GatewayIPAddr
	record, ok := p.Recordsv4[req.ClientHWAddr.String()]
	hostname := req.HostName()
	cinfo := req.Options.Get(dhcpv4.OptionUserClassInformation)
	if !ok {
		// Allocating new address since there isn't one allocated
		log.Printf("MAC address %s is new, leasing new IPv4 address (giaddr=%s)", req.ClientHWAddr.String(), giaddr)

		// Select the appropriate allocator based on configuration
		var allocator allocators.Allocator
		var cidr string
		var err error

		if p.useSubnetPools {
			allocator, cidr, err = p.subnetPoolMgr.GetAllocatorForGiaddr(giaddr)
			if err != nil {
				log.Errorf("Could not find allocator for giaddr %s: %v", giaddr, err)
				return nil, true
			}
			log.Debugf("using allocator for subnet %s (giaddr=%s)", cidr, giaddr)
		} else {
			allocator = p.allocator
		}

		ip, err := allocator.Allocate(net.IPNet{})
		if err != nil {
			log.Errorf("Could not allocate IP for MAC %s: %v", req.ClientHWAddr.String(), err)
			return nil, true
		}
		rec := Record{
			IP:       ip.IP.To4(),
			expires:  int(time.Now().Add(p.LeaseTime).Unix()),
			hostname: hostname,
		}
		err = p.saveIPAddress(req.ClientHWAddr, &rec)
		if err != nil {
			log.Errorf("SaveIPAddress for MAC %s failed: %v", req.ClientHWAddr.String(), err)
		}
		p.Recordsv4[req.ClientHWAddr.String()] = &rec
		record = &rec
		resp.YourIPAddr = record.IP
		resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(p.LeaseTime.Round(time.Second)))
		if p.useSubnetPools {
			log.Infof("assigning %s to %s from subnet %s with a lease duration of %s", record.IP, req.ClientHWAddr.String(), cidr, p.LeaseTime)
		} else {
			log.Infof("assigning %s to %s with a lease duration of %s", record.IP, req.ClientHWAddr.String(), p.LeaseTime)
		}

		if string(cinfo) != "iPXE" {
			// BOOT STAGE 1: Send iPXE bootloader over TFTP
			resp, _ = ipxe.ServeIPXEBootloader(log, req, resp)
		}
	} else {
		if string(cinfo) == "iPXE" {
			// BOOT STAGE 2: Send URL to BSS boot script
			resp.Options.Update(dhcpv4.OptBootFileName(globalConfig.scriptPath))
			resp.YourIPAddr = record.IP
			resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(p.LeaseTime.Round(time.Second)))
		} else {
			// At this point, the client already has already obtained a lease and is probably
			// requesting to renew it. The client needs to go through the full DHCP handshake
			// so it can be determined if it has been discovered, so we send a DHCPNAK to
			// initiate this.
			var err error
			resp, err = dhcpv4.New(
				dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
				dhcpv4.WithTransactionID(req.TransactionID),
				dhcpv4.WithHwAddr(req.ClientHWAddr),
				dhcpv4.WithServerIP(resp.ServerIPAddr),
			)
			if err != nil {
				log.Errorf("failed to create new %s message: %s", dhcpv4.MessageTypeNak, err)
				return resp, true
			}
			err = p.deleteIPAddress(req.ClientHWAddr)
			if err != nil {
				log.Errorf("DeleteIPAddress for MAC %s failed: %v", req.ClientHWAddr.String(), err)
			}
			delete(p.Recordsv4, req.ClientHWAddr.String())

			// Free the IP from the appropriate allocator
			if p.useSubnetPools {
				// Find which pool this IP belongs to
				for cidr, pool := range p.subnetPoolMgr.Pools {
					if pool.CIDR.Contains(record.IP) {
						if err := pool.Allocator.Free(net.IPNet{IP: record.IP}); err != nil {
							log.Warnf("unable to delete IP %s from subnet %s: %s", record.IP.String(), cidr, err)
						}
						break
					}
				}
			} else {
				if err := p.allocator.Free(net.IPNet{IP: record.IP}); err != nil {
					log.Warnf("unable to delete IP %s: %s", record.IP.String(), err)
				}
			}
			log.Printf("MAC %s already exists with IP %s, sending %s to reinitiate DHCP handshake", req.ClientHWAddr.String(), record.IP, dhcpv4.MessageTypeNak)
		}
	}

	debug.DebugResponse(log, resp)
	return resp, true
}
