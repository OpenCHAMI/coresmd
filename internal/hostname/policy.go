package hostname

import (
	"net"
)

type Policy struct {
	DefaultPattern string            // may be ""
	ByType         map[string]string // Component.Type -> pattern
}

// TODO: Implement functions and test for this
type SubnetPolicy struct {
	Subnet *net.IPNet
	Policy Policy
}

// HostnameFor is a wrapper around ExpandHostnamePattern that maps a
// Component.Type with a pattern. If a Component.Type is not found, the
// policy.DefaultPattern is used instead. If no policy.DefaultPattern is
// set, this function will return an empty string and a boolean indicating
// that it failed to expand the hostname properly.
//
// See the ExpandHostnamePattern description for details on notation and
// how patterns are expanded.
func (p Policy) HostnameFor(componentType string, nid int64, id string) (string, bool) {
	pat := ""
	if p.ByType != nil {
		pat = p.ByType[componentType]
	}
	if pat == "" {
		pat = p.DefaultPattern
	}
	if pat == "" {
		return "", false
	}
	return ExpandHostnamePattern(pat, nid, id), true
}
