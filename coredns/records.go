package coredns

import (
	"fmt"
	"net"
	"strings"

	"github.com/OpenCHAMI/coresmd/coresmd"
	"github.com/miekg/dns"
)

// RecordGenerator handles DNS record generation for the coresmd plugin
type RecordGenerator struct {
	zones []Zone
	cache *coresmd.Cache
}

// NewRecordGenerator creates a new record generator
func NewRecordGenerator(zones []Zone, cache *coresmd.Cache) *RecordGenerator {
	return &RecordGenerator{
		zones: zones,
		cache: cache,
	}
}

// GenerateARecord generates an A record for the given hostname
func (rg *RecordGenerator) GenerateARecord(hostname string) *dns.A {
	ip := rg.lookupIPForHostname(hostname)
	if ip == nil {
		return nil
	}

	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(hostname),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: ip,
	}
}

// GeneratePTRRecord generates a PTR record for the given reverse lookup name
func (rg *RecordGenerator) GeneratePTRRecord(reverseName string) *dns.PTR {
	hostname := rg.lookupHostnameForIP(reverseName)
	if hostname == "" {
		return nil
	}

	return &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(reverseName),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Ptr: dns.Fqdn(hostname),
	}
}

// GenerateCNAMERecord generates a CNAME record for the given alias
func (rg *RecordGenerator) GenerateCNAMERecord(alias, target string) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(alias),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Target: dns.Fqdn(target),
	}
}

// GenerateTXTRecord generates a TXT record with component metadata
func (rg *RecordGenerator) GenerateTXTRecord(hostname string) *dns.TXT {
	metadata := rg.getComponentMetadata(hostname)
	if len(metadata) == 0 {
		return nil
	}

	return &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(hostname),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Txt: metadata,
	}
}

// lookupIPForHostname finds the IP address for a given hostname
func (rg *RecordGenerator) lookupIPForHostname(hostname string) net.IP {
	if rg.cache == nil {
		return nil
	}

	rg.cache.Mutex.RLock()
	defer rg.cache.Mutex.RUnlock()

	// Check each zone for node or BMC patterns
	for _, zone := range rg.zones {
		// Node pattern: e.g., nid0001.cluster.local
		if strings.HasSuffix(hostname, zone.Name) && zone.NodePattern != "" {
			for _, ei := range rg.cache.EthernetInterfaces {
				if comp, ok := rg.cache.Components[ei.ComponentID]; ok && comp.Type == "Node" {
					expectedHost := strings.Replace(zone.NodePattern, "{04d}", fmt.Sprintf("%04d", comp.NID), 1)
					expectedHostFQDN := expectedHost + "." + zone.Name
					if hostname == expectedHostFQDN {
						if len(ei.IPAddresses) > 0 {
							return net.ParseIP(ei.IPAddresses[0].IPAddress)
						}
					}
				}
			}
		}
		// BMC pattern: e.g., bmc-xname.cluster.local
		if strings.HasSuffix(hostname, zone.Name) && zone.BMCPattern != "" {
			for _, ei := range rg.cache.EthernetInterfaces {
				if comp, ok := rg.cache.Components[ei.ComponentID]; ok && comp.Type == "NodeBMC" {
					expectedHost := strings.Replace(zone.BMCPattern, "{id}", comp.ID, 1)
					expectedHostFQDN := expectedHost + "." + zone.Name
					if hostname == expectedHostFQDN {
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

// lookupHostnameForIP finds the hostname for a given IP address
func (rg *RecordGenerator) lookupHostnameForIP(reverseName string) string {
	if rg.cache == nil {
		return ""
	}

	rg.cache.Mutex.RLock()
	defer rg.cache.Mutex.RUnlock()

	// Convert reverse name to IP
	if ip := reverseToIP(reverseName); ip != nil {
		// Find matching EthernetInterface
		for _, ei := range rg.cache.EthernetInterfaces {
			for _, ipEntry := range ei.IPAddresses {
				if net.ParseIP(ipEntry.IPAddress).Equal(ip) {
					if comp, ok := rg.cache.Components[ei.ComponentID]; ok {
						// Return node or BMC hostname
						for _, zone := range rg.zones {
							if comp.Type == "Node" && zone.NodePattern != "" {
								host := strings.Replace(zone.NodePattern, "{04d}", fmt.Sprintf("%04d", comp.NID), 1)
								return host + "." + zone.Name
							}
							if comp.Type == "NodeBMC" && zone.BMCPattern != "" {
								host := strings.Replace(zone.BMCPattern, "{id}", comp.ID, 1)
								return host + "." + zone.Name
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// getComponentMetadata returns metadata strings for a component
func (rg *RecordGenerator) getComponentMetadata(hostname string) []string {
	if rg.cache == nil {
		return nil
	}

	rg.cache.Mutex.RLock()
	defer rg.cache.Mutex.RUnlock()

	// Find component for the hostname
	for _, zone := range rg.zones {
		if strings.HasSuffix(hostname, zone.Name) {
			// Check for node pattern
			if zone.NodePattern != "" {
				for _, ei := range rg.cache.EthernetInterfaces {
					if comp, ok := rg.cache.Components[ei.ComponentID]; ok && comp.Type == "Node" {
						expectedHost := strings.Replace(zone.NodePattern, "{04d}", fmt.Sprintf("%04d", comp.NID), 1)
						expectedHostFQDN := expectedHost + "." + zone.Name
						if hostname == expectedHostFQDN {
							return []string{
								fmt.Sprintf("type=%s", comp.Type),
								fmt.Sprintf("nid=%d", comp.NID),
								fmt.Sprintf("component_id=%s", comp.ID),
								fmt.Sprintf("mac=%s", ei.MACAddress),
							}
						}
					}
				}
			}
			// Check for BMC pattern
			if zone.BMCPattern != "" {
				for _, ei := range rg.cache.EthernetInterfaces {
					if comp, ok := rg.cache.Components[ei.ComponentID]; ok && comp.Type == "NodeBMC" {
						expectedHost := strings.Replace(zone.BMCPattern, "{id}", comp.ID, 1)
						expectedHostFQDN := expectedHost + "." + zone.Name
						if hostname == expectedHostFQDN {
							return []string{
								fmt.Sprintf("type=%s", comp.Type),
								fmt.Sprintf("component_id=%s", comp.ID),
								fmt.Sprintf("mac=%s", ei.MACAddress),
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// GenerateAllRecordsForHostname generates all available records for a hostname
func (rg *RecordGenerator) GenerateAllRecordsForHostname(hostname string) []dns.RR {
	var records []dns.RR

	// Generate A record
	if aRecord := rg.GenerateARecord(hostname); aRecord != nil {
		records = append(records, aRecord)
	}

	// Generate TXT record
	if txtRecord := rg.GenerateTXTRecord(hostname); txtRecord != nil {
		records = append(records, txtRecord)
	}

	return records
}

// GenerateAllRecordsForIP generates all available records for an IP address
func (rg *RecordGenerator) GenerateAllRecordsForIP(reverseName string) []dns.RR {
	var records []dns.RR

	// Generate PTR record
	if ptrRecord := rg.GeneratePTRRecord(reverseName); ptrRecord != nil {
		records = append(records, ptrRecord)
	}

	return records
}
