// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package plugin

import (
	"crypto/sha256"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin/transfer"
	"github.com/miekg/dns"
)

// Record TTL and SOA timers used for outgoing zone transfers.
const (
	xfrTTL        = 60
	soaRefresh    = 300
	soaRetry      = 60
	soaExpire     = 604800
	soaMinimum    = 60
	xfrDefaultNS  = "ns"
	xfrDefaultMbx = "hostmaster"
)

// xfrState is shared between all copies of Plugin (Plugin is passed by value
// in the handler chain, so this must be a pointer).
type xfrState struct {
	mu     sync.RWMutex
	serial uint32
	hash   [32]byte
	xfer   *transfer.Transfer
}

// Transfer implements transfer.Transferer. It answers AXFR for every zone
// configured in the coresmd block, and does an AXFR fallback for IXFR
// (serial != 0) unless the serial matches, in which case only the SOA is sent.
func (p Plugin) Transfer(zone string, serial uint32) (<-chan []dns.RR, error) {
	z := p.findConfiguredZone(zone)
	if z == nil {
		return nil, transfer.ErrNotAuthoritative
	}
	if p.cache == nil || p.xfr == nil {
		return nil, transfer.ErrNotAuthoritative
	}

	p.xfr.mu.RLock()
	cur := p.xfr.serial
	p.xfr.mu.RUnlock()

	soa := p.soaRecord(z, cur)
	ch := make(chan []dns.RR)

	go func() {
		defer close(ch)
		if serial != 0 && serial == cur {
			// IXFR with up-to-date serial: single SOA means "nothing changed".
			ch <- []dns.RR{soa}
			return
		}
		ch <- []dns.RR{soa, p.nsRecord(z)}
		rrs := p.zoneRecords(z)
		// Send in modest batches; the transfer plugin packs them into envelopes.
		for i := 0; i < len(rrs); i += 100 {
			end := i + 100
			if end > len(rrs) {
				end = len(rrs)
			}
			ch <- rrs[i:end]
		}
		ch <- []dns.RR{soa}
	}()

	return ch, nil
}

// findConfiguredZone returns the configured zone whose name equals zone
// (exact apex match, not sub-domain match). Zone transfers are only valid
// at the apex.
func (p Plugin) findConfiguredZone(zone string) *Zone {
	want := strings.TrimSuffix(strings.ToLower(zone), ".")
	for i := range p.zones {
		if strings.TrimSuffix(strings.ToLower(p.zones[i].Name), ".") == want {
			return &p.zones[i]
		}
	}
	return nil
}

func fqdn(name string) string {
	return dns.Fqdn(strings.ToLower(name))
}

func (p Plugin) nsName(z *Zone) string {
	if z.NS != "" {
		return fqdn(z.NS)
	}
	return fqdn(xfrDefaultNS + "." + z.Name)
}

func (p Plugin) mboxName(z *Zone) string {
	if z.Mailbox != "" {
		return fqdn(z.Mailbox)
	}
	return fqdn(xfrDefaultMbx + "." + z.Name)
}

func (p Plugin) soaRecord(z *Zone, serial uint32) *dns.SOA {
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: fqdn(z.Name), Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: xfrTTL},
		Ns:      p.nsName(z),
		Mbox:    p.mboxName(z),
		Serial:  serial,
		Refresh: soaRefresh,
		Retry:   soaRetry,
		Expire:  soaExpire,
		Minttl:  soaMinimum,
	}
}

func (p Plugin) nsRecord(z *Zone) *dns.NS {
	return &dns.NS{
		Hdr: dns.RR_Header{Name: fqdn(z.Name), Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: xfrTTL},
		Ns:  p.nsName(z),
	}
}

// zoneRecords builds the full A/AAAA record set for one zone from the SMD
// cache, using the same naming rules as lookupA/lookupAAAA:
//   - Node:    <nid-pattern>.<zone> and <xname>.<zone>
//   - NodeBMC: <xname>.<zone>
//
// The result is sorted so the output (and its hash) is deterministic.
func (p Plugin) zoneRecords(z *Zone) []dns.RR {
	var rrs []dns.RR
	if p.cache == nil {
		return rrs
	}

	p.cache.Mutex.RLock()
	defer p.cache.Mutex.RUnlock()

	seen := make(map[string]struct{})
	add := func(name string, ip net.IP) {
		hdrName := fqdn(name)
		key := hdrName + "|" + ip.String()
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		if ip4 := ip.To4(); ip4 != nil {
			rrs = append(rrs, &dns.A{
				Hdr: dns.RR_Header{Name: hdrName, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: xfrTTL},
				A:   ip4,
			})
			return
		}
		rrs = append(rrs, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: hdrName, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: xfrTTL},
			AAAA: ip.To16(),
		})
	}

	for _, ei := range p.cache.EthernetInterfaces {
		comp, ok := p.cache.Components[ei.ComponentID]
		if !ok {
			continue
		}
		var names []string
		switch comp.Type {
		case "Node":
			names = append(names, comp.ID+"."+z.Name)
			if z.NodePattern != "" {
				names = append(names, expandPattern(z.NodePattern, comp.NID, comp.ID)+"."+z.Name)
			}
		case "NodeBMC":
			names = append(names, comp.ID+"."+z.Name)
		default:
			continue
		}
		for _, ipEntry := range ei.IPAddresses {
			ip := net.ParseIP(ipEntry.IPAddress)
			if ip == nil {
				continue
			}
			for _, n := range names {
				add(n, ip)
			}
		}
	}

	sort.Slice(rrs, func(i, j int) bool { return rrs[i].String() < rrs[j].String() })
	return rrs
}

// zoneHash returns a digest of the complete record set across all zones.
func (p Plugin) zoneHash() [32]byte {
	h := sha256.New()
	for i := range p.zones {
		for _, rr := range p.zoneRecords(&p.zones[i]) {
			h.Write([]byte(rr.String()))
			h.Write([]byte{'\n'})
		}
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// startSerialWatcher bumps the SOA serial whenever the record set changes and
// sends NOTIFY to the hosts listed in the transfer plugin's "to" directive.
// It runs at the same cadence as the SMD cache refresh.
func (p Plugin) startSerialWatcher(interval time.Duration) {
	if p.xfr == nil {
		return
	}
	update := func() {
		h := p.zoneHash()
		p.xfr.mu.Lock()
		changed := h != p.xfr.hash
		if changed {
			s := uint32(time.Now().Unix())
			if s <= p.xfr.serial {
				s = p.xfr.serial + 1
			}
			p.xfr.serial = s
			p.xfr.hash = h
		}
		serial := p.xfr.serial
		xfer := p.xfr.xfer
		p.xfr.mu.Unlock()

		if changed {
			log.Infof("zone data changed, SOA serial is now %d", serial)
			if xfer != nil {
				for i := range p.zones {
					if err := xfer.Notify(fqdn(p.zones[i].Name)); err != nil {
						log.Warnf("NOTIFY for %s failed: %v", p.zones[i].Name, err)
					}
				}
			}
		}
	}

	go func() {
		// Give the cache a moment to do its initial fill before the first hash.
		time.Sleep(2 * time.Second)
		update()
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			update()
		}
	}()
}
