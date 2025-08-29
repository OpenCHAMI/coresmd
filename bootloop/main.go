package bootloop

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
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
	Recordsv4 map[string]*Record
	LeaseTime time.Duration
	leasedb   *sql.DB
	allocator allocators.Allocator
}

var log = logger.GetLogger("plugins/bootloop")

var (
	ipv4Start  net.IP
	ipv4End    net.IP
	ipv4Range  int
	p          PluginState
	scriptPath string
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

	// Ensure all required args were passed
	if len(args) != 5 {
		return nil, fmt.Errorf("wanted 5 arguments (file name, iPXE script path, lease duration, IPv4 range start, IPv4 range end), got %d", len(args))
	}
	var err error

	// Parse file name
	// Check other plugin args before trying to setup actual storage
	filename := args[0]
	if filename == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Parse boot script path
	scriptPath = args[1]
	if filename == "" {
		return nil, fmt.Errorf("script path cannot be empty; use 'default' if unsure")
	}

	// Parse short lease duration
	p.LeaseTime, err = time.ParseDuration(args[2])
	if err != nil {
		return nil, fmt.Errorf("failed to parse short lease duration %q: %w", args[0], err)
	}

	// Parse start IP
	ipv4Start := net.ParseIP(args[3])
	if ipv4Start.To4() == nil {
		return nil, fmt.Errorf("invalid IPv4 address for range start: %s", args[1])
	}

	// Parse end IP
	ipv4End := net.ParseIP(args[4])
	if ipv4End.To4() == nil {
		return nil, fmt.Errorf("invalid IPv4 address for range end: %s", args[2])
	}

	// Calculate range to make sure it is valid
	if binary.BigEndian.Uint32(ipv4Start.To4()) > binary.BigEndian.Uint32(ipv4End.To4()) {
		return nil, fmt.Errorf("start IP must be equal or higher than end IP")
	}
	log.Infof("IPv4 address range from %s to %s", ipv4Start.To4().String(), ipv4End.To4().String())
	ipv4Range := binary.BigEndian.Uint32(ipv4End.To4()) - binary.BigEndian.Uint32(ipv4Start.To4()) + 1
	log.Infof("%d addresses in range", ipv4Range)

	// Create IP address allocator based on IP range
	p.allocator, err = bitmap.NewIPv4Allocator(ipv4Start, ipv4End)
	if err != nil {
		return nil, fmt.Errorf("failed to create an allocator: %w", err)
	}

	// Set up storage backend using passed file path
	if err := p.registerBackingDB(filename); err != nil {
		return nil, fmt.Errorf("failed to setup lease storage: %w", err)
	}
	p.Recordsv4, err = loadRecords(p.leasedb)
	if err != nil {
		return nil, fmt.Errorf("failed to load records from file: %v", err)
	}

	log.Info("bootloop plugin initialized")

	// Allocate any pre-existing leases
	for _, v := range p.Recordsv4 {
		ip, err := p.allocator.Allocate(net.IPNet{IP: v.IP})
		if err != nil {
			return nil, fmt.Errorf("failed to re-allocate leased ip %v: %v", v.IP.String(), err)
		}
		if ip.IP.String() != v.IP.String() {
			return nil, fmt.Errorf("allocator did not re-allocate requested leased ip %v: %v", v.IP.String(), ip.String())
		}
	}

	return p.Handler4, nil
}

func (p *PluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	// Make sure db doesn't get updated while reading
	p.Lock()
	defer p.Unlock()

	debug.DebugRequest(log, req)

	// Set root path to this server's IP
	resp.Options.Update(dhcpv4.OptRootPath(resp.ServerIPAddr.String()))

	record, ok := p.Recordsv4[req.ClientHWAddr.String()]
	hostname := req.HostName()
	cinfo := req.Options.Get(dhcpv4.OptionUserClassInformation)
	if !ok {
		// Allocating new address since there isn't one allocated
		log.Printf("MAC address %s is new, leasing new IPv4 address", req.ClientHWAddr.String())
		ip, err := p.allocator.Allocate(net.IPNet{})
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
		log.Infof("assigning %s to %s with a lease duration of %s", record.IP, req.ClientHWAddr.String(), p.LeaseTime)

		if string(cinfo) != "iPXE" {
			// BOOT STAGE 1: Send iPXE bootloader over TFTP
			resp, _ = ipxe.ServeIPXEBootloader(log, req, resp)
		}
	} else {
		if string(cinfo) == "iPXE" {
			// BOOT STAGE 2: Send URL to BSS boot script
			resp.Options.Update(dhcpv4.OptBootFileName(scriptPath))
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
			if err := p.allocator.Free(net.IPNet{IP: record.IP}); err != nil {
				log.Warnf("unable to delete IP %s: %s", record.IP.String(), err)
			}
			log.Printf("MAC %s already exists with IP %s, sending %s to reinitiate DHCP handshake", req.ClientHWAddr.String(), record.IP, dhcpv4.MessageTypeNak)
		}
	}

	debug.DebugResponse(log, resp)
	return resp, true
}
