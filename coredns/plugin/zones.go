package plugin

import "strings"

// Zone represents a DNS zone configuration
type Zone struct {
	Name        string // Zone name (e.g., "cluster.local")
	NodePattern string // Pattern for node records (e.g., "nid{04d}.cluster.local")
}

// ZoneManager handles zone operations and record lookups
type ZoneManager struct {
	zones []Zone
}

// NewZoneManager creates a new zone manager
func NewZoneManager(zones []Zone) *ZoneManager {
	return &ZoneManager{
		zones: zones,
	}
}

// FindZone finds the appropriate zone for a given domain name
func (zm *ZoneManager) FindZone(domain string) *Zone {
	for i := range zm.zones {
		if isSubdomain(domain, zm.zones[i].Name) {
			return &zm.zones[i]
		}
	}
	return nil
}

// isSubdomain checks if domain is a subdomain of zone.
// It normalizes both domain and zone to ensure trailing dots and lowercase.
// Returns true if domain is equal to zone or ends with ".zone".
func isSubdomain(domain, zone string) bool {
	d := strings.TrimSuffix(strings.ToLower(domain), ".")
	z := strings.TrimSuffix(strings.ToLower(zone), ".")

	if d == z {
		return true
	}
	return strings.HasSuffix(d, "."+z)
}
