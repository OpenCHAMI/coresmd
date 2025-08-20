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

	// Handle A record queries
	if qType == dns.TypeA {
		if ip := p.lookupA(qName); ip != nil {
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
			w.WriteMsg(msg)

			// Record metrics
			RequestCount.WithLabelValues(server, zone, "A").Inc()
			CacheHits.WithLabelValues(server, zone, "A").Inc()
			RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

			return dns.RcodeSuccess, nil
		}
		// Cache miss for A record
		CacheMisses.WithLabelValues(server, zone, "A").Inc()
	}

	// Handle PTR record queries (reverse lookups)
	if qType == dns.TypePTR {
		if ptr := p.lookupPTR(qName); ptr != "" {
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
			w.WriteMsg(msg)

			// Record metrics
			RequestCount.WithLabelValues(server, zone, "PTR").Inc()
			CacheHits.WithLabelValues(server, zone, "PTR").Inc()
			RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

			return dns.RcodeSuccess, nil
		}
		// Cache miss for PTR record
		CacheMisses.WithLabelValues(server, zone, "PTR").Inc()
	}

	// Record metrics for other query types
	RequestCount.WithLabelValues(server, zone, "other").Inc()
	RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

	// Fall through to the next plugin
	return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
}

// lookupA tries to find an A record for the given name using the SMD cache and zones
func (p *Plugin) lookupA(name string) net.IP {
	if p.cache == nil {
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
						if len(ei.IPAddresses) > 0 {
							return net.ParseIP(ei.IPAddresses[0].IPAddress)
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
						if len(ei.IPAddresses) > 0 {
							return net.ParseIP(ei.IPAddresses[0].IPAddress)
						}
					}
				}
			}
		}
	}
	return nil
}

// lookupPTR tries to find a PTR record for the given reverse lookup name
func (p *Plugin) lookupPTR(name string) string {
	if p.cache == nil {
		return ""
	}
	p.cache.Mutex.RLock()
	defer p.cache.Mutex.RUnlock()

	// Convert reverse name to IP
	if ip := reverseToIP(name); ip != nil {
		// Find matching EthernetInterface
		for _, ei := range p.cache.EthernetInterfaces {
			for _, ipEntry := range ei.IPAddresses {
				if net.ParseIP(ipEntry.IPAddress).Equal(ip) {
					if comp, ok := p.cache.Components[ei.ComponentID]; ok {
						// Return node or BMC hostname
						for _, zone := range p.zones {
							if comp.Type == "Node" && zone.NodePattern != "" {
								// host := strings.Replace(zone.NodePattern, "{04d}", fmt.Sprintf("%04d", comp.NID), 1)
								return comp.ID + "." + zone.Name
							}
							if comp.Type == "NodeBMC" && zone.BMCPattern != "" {
								// host := strings.Replace(zone.BMCPattern, "{id}", comp.ID, 1)
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

// reverseToIP converts a reverse DNS name to an IP address (IPv4 only)
func reverseToIP(name string) net.IP {
	// Remove trailing dot if present
	name = strings.TrimSuffix(name, ".")
	const suffix = ".in-addr.arpa"
	if !strings.HasSuffix(name, suffix) {
		return nil
	}
	trimmed := strings.TrimSuffix(name, suffix)
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
