package plugin

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

// ServeDNS handles DNS requests for the coresmd plugin
func (p Plugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	start := time.Now()
	server := "default" // Use default server name for metrics

	if len(r.Question) == 0 {
		RequestCount.WithLabelValues(server, "unknown", "empty").Inc()
		RequestDuration.WithLabelValues(server, "unknown").Observe(time.Since(start).Seconds())
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
	}

	q := r.Question[0]
	qName := strings.TrimSuffix(q.Name, ".")
	qType := q.Qtype

	// Determine zone for metrics
	zone := "unknown"
	for _, z := range p.zones {
		if strings.HasSuffix(qName, z.Name) {
			zone = z.Name
			break
		}
	}

	// Handle DNS queries based on type
	switch qType {
	case dns.TypeA:
		// Handle A record queries (IPv4)
		if ip := p.lookupA(qName); ip != nil {
			log.Debugf("A record lookup succeeded: %s -> %s", qName, ip)
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative = true
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: ip,
			}
			msg.Answer = append(msg.Answer, rr)
			if err := w.WriteMsg(msg); err != nil {
				log.Errorf("Failed to write A record response for %s: %v", qName, err)
				return dns.RcodeServerFailure, err
			}

			// Record metrics
			RequestCount.WithLabelValues(server, zone, "A").Inc()
			CacheHits.WithLabelValues(server, zone, "A").Inc()
			RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

			return dns.RcodeSuccess, nil
		}
		// Cache miss for A record
		log.Debugf("A record cache miss for %s in zone %s", qName, zone)
		CacheMisses.WithLabelValues(server, zone, "A").Inc()

	case dns.TypeAAAA:
		// Handle AAAA record queries (IPv6)
		if ip := p.lookupAAAA(qName); ip != nil {
			log.Debugf("AAAA record lookup succeeded: %s -> %s", qName, ip)
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative = true
			rr := &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				AAAA: ip,
			}
			msg.Answer = append(msg.Answer, rr)
			if err := w.WriteMsg(msg); err != nil {
				log.Errorf("Failed to write AAAA record response for %s: %v", qName, err)
				return dns.RcodeServerFailure, err
			}

			// Record metrics
			RequestCount.WithLabelValues(server, zone, "AAAA").Inc()
			CacheHits.WithLabelValues(server, zone, "AAAA").Inc()
			RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

			return dns.RcodeSuccess, nil
		}
		// Cache miss for AAAA record
		log.Debugf("AAAA record cache miss for %s in zone %s", qName, zone)
		CacheMisses.WithLabelValues(server, zone, "AAAA").Inc()

	case dns.TypePTR:
		// Handle PTR record queries (reverse lookups)
		if ptr := p.lookupPTR(qName); ptr != "" {
			log.Debugf("PTR record lookup succeeded: %s -> %s", qName, ptr)
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative = true
			rr := &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Ptr: dns.Fqdn(ptr),
			}
			msg.Answer = append(msg.Answer, rr)
			if err := w.WriteMsg(msg); err != nil {
				log.Errorf("Failed to write PTR record response for %s: %v", qName, err)
				return dns.RcodeServerFailure, err
			}

			// Record metrics
			RequestCount.WithLabelValues(server, zone, "PTR").Inc()
			CacheHits.WithLabelValues(server, zone, "PTR").Inc()
			RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

			return dns.RcodeSuccess, nil
		}
		// Cache miss for PTR record
		log.Debugf("PTR record cache miss for %s in zone %s", qName, zone)
		CacheMisses.WithLabelValues(server, zone, "PTR").Inc()

	default:
		// Record metrics for other query types
		RequestCount.WithLabelValues(server, zone, "other").Inc()
	}

	RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

	// Fall through to the next plugin
	log.Debugf("No match found for %s (type %s), passing to next plugin", qName, dns.TypeToString[qType])
	return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
}

// lookupA tries to find an A record (IPv4) for the given name using the SMD cache and zones
func (p *Plugin) lookupA(name string) net.IP {
	if p.cache == nil {
		log.Warn("Cache is nil during A record lookup")
		return nil
	}
	p.cache.Mutex.RLock()
	defer p.cache.Mutex.RUnlock()

	for _, zone := range p.zones {
		if strings.HasSuffix(name, zone.Name) {
			for _, ei := range p.cache.EthernetInterfaces {
				if comp, ok := p.cache.Components[ei.ComponentID]; ok && comp.Type == "Node" {
					xnameHost := comp.ID
					xnameFQDN := xnameHost + "." + zone.Name
					// Expand node pattern
					nidFQDN := ""
					if zone.NodePattern != "" {
						// nid{04d} pattern: e.g., nid0001.cluster.local
						nidHost := expandPattern(zone.NodePattern, comp.NID, comp.ID)
						nidFQDN = nidHost + "." + zone.Name
					}

					if name == nidFQDN || name == xnameFQDN {
						for _, ipEntry := range ei.IPAddresses {
							if ip := net.ParseIP(ipEntry.IPAddress); ip != nil && ip.To4() != nil {
								return ip
							}
						}
					}
				}
			}
		}
		if strings.HasSuffix(name, zone.Name) {
			for _, ei := range p.cache.EthernetInterfaces {
				if comp, ok := p.cache.Components[ei.ComponentID]; ok && comp.Type == "NodeBMC" {
					xnameHost := comp.ID
					xnameFQDN := xnameHost + "." + zone.Name
					if name == xnameFQDN {
						for _, ipEntry := range ei.IPAddresses {
							if ip := net.ParseIP(ipEntry.IPAddress); ip != nil && ip.To4() != nil {
								return ip
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// lookupAAAA tries to find an AAAA record (IPv6) for the given name using the SMD cache and zones
func (p *Plugin) lookupAAAA(name string) net.IP {
	if p.cache == nil {
		log.Warn("Cache is nil during AAAA record lookup")
		return nil
	}
	p.cache.Mutex.RLock()
	defer p.cache.Mutex.RUnlock()

	for _, zone := range p.zones {
		if strings.HasSuffix(name, zone.Name) {
			for _, ei := range p.cache.EthernetInterfaces {
				if comp, ok := p.cache.Components[ei.ComponentID]; ok && comp.Type == "Node" {
					xnameHost := comp.ID
					xnameFQDN := xnameHost + "." + zone.Name
					// Expand node pattern
					nidFQDN := ""
					if zone.NodePattern != "" {
						nidHost := expandPattern(zone.NodePattern, comp.NID, comp.ID)
						nidFQDN = nidHost + "." + zone.Name
					}

					if name == nidFQDN || name == xnameFQDN {
						for _, ipEntry := range ei.IPAddresses {
							if ip := net.ParseIP(ipEntry.IPAddress); ip != nil && ip.To4() == nil && ip.To16() != nil {
								return ip
							}
						}
					}
				}
			}
		}
		if strings.HasSuffix(name, zone.Name) {
			for _, ei := range p.cache.EthernetInterfaces {
				if comp, ok := p.cache.Components[ei.ComponentID]; ok && comp.Type == "NodeBMC" {
					xnameHost := comp.ID
					xnameFQDN := xnameHost + "." + zone.Name
					if name == xnameFQDN {
						for _, ipEntry := range ei.IPAddresses {
							if ip := net.ParseIP(ipEntry.IPAddress); ip != nil && ip.To4() == nil && ip.To16() != nil {
								return ip
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// lookupPTR tries to find a PTR record for the given reverse lookup name (both IPv4 and IPv6)
func (p *Plugin) lookupPTR(name string) string {
	if p.cache == nil {
		log.Warn("Cache is nil during PTR record lookup")
		return ""
	}
	p.cache.Mutex.RLock()
	defer p.cache.Mutex.RUnlock()

	// Convert reverse name to IP (handles both IPv4 and IPv6)
	if ip := reverseToIP(name); ip != nil {
		// Find matching EthernetInterface
		for _, ei := range p.cache.EthernetInterfaces {
			for _, ipEntry := range ei.IPAddresses {
				if net.ParseIP(ipEntry.IPAddress).Equal(ip) {
					if comp, ok := p.cache.Components[ei.ComponentID]; ok {
						// Return node or BMC hostname
						for _, zone := range p.zones {
							if comp.Type == "Node" && zone.NodePattern != "" {
								return comp.ID + "." + zone.Name
							}
							if comp.Type == "NodeBMC" {
								return comp.ID + "." + zone.Name
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// reverseToIP converts a reverse DNS name to an IP address (supports both IPv4 and IPv6)
func reverseToIP(name string) net.IP {
	// Remove trailing dot if present
	name = strings.TrimSuffix(name, ".")

	// Try IPv4 reverse lookup (in-addr.arpa)
	const ipv4Suffix = ".in-addr.arpa"
	if strings.HasSuffix(name, ipv4Suffix) {
		trimmed := strings.TrimSuffix(name, ipv4Suffix)
		parts := strings.Split(trimmed, ".")
		if len(parts) != 4 {
			return nil
		}
		// Reverse the order
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		return net.ParseIP(strings.Join(parts, "."))
	}

	// Try IPv6 reverse lookup (ip6.arpa)
	const ipv6Suffix = ".ip6.arpa"
	if strings.HasSuffix(name, ipv6Suffix) {
		trimmed := strings.TrimSuffix(name, ipv6Suffix)
		parts := strings.Split(trimmed, ".")
		if len(parts) != 32 {
			return nil
		}
		// Reverse the nibbles to reconstruct the IPv6 address
		var ipv6Str string
		for i := len(parts) - 1; i >= 0; i-- {
			ipv6Str += parts[i]
			if (len(parts)-i)%4 == 0 && i > 0 {
				ipv6Str += ":"
			}
		}
		return net.ParseIP(ipv6Str)
	}

	return nil
}

// expandPattern replaces {Nd} with zero-padded NID and {id} with xname
func expandPattern(pattern string, nid int64, id string) string {
	out := strings.ReplaceAll(pattern, "{id}", id)
	re := regexp.MustCompile(`\{0*(\d+)d\}`)
	out = re.ReplaceAllStringFunc(out, func(m string) string {
		nStr := re.FindStringSubmatch(m)[1]
		n, _ := strconv.Atoi(nStr)
		return fmt.Sprintf("%0*d", n, nid)
	})
	return out
}

// Name returns the plugin name
func (p Plugin) Name() string {
	return "coresmd"
}
