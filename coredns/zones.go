package coredns

// Zone represents a DNS zone configuration
type Zone struct {
	Name        string // Zone name (e.g., "cluster.local")
	NodePattern string // Pattern for node records (e.g., "nid{04d}.cluster.local")
	BMCPattern  string // Pattern for BMC records (e.g., "bmc-{id}.cluster.local")
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
	for _, zone := range zm.zones {
		if isSubdomain(domain, zone.Name) {
			return &zone
		}
	}
	return nil
}

// isSubdomain checks if domain is a subdomain of zone
func isSubdomain(domain, zone string) bool {
	// Simple subdomain check - in a real implementation, this would be more sophisticated
	if len(domain) <= len(zone) {
		return false
	}

	// Check if domain ends with zone
	if len(domain) > len(zone) && domain[len(domain)-len(zone)-1] == '.' {
		return domain[len(domain)-len(zone):] == zone
	}

	return false
}
