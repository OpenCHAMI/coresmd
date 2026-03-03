package hostname

import (
	"net"
	"testing"
)

func TestPolicyHostnameFor(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		wants  []string
	}{

		{
			name: "policy_with_type_patterns",
			policy: Policy{
				DefaultPattern: "",
				ByType: map[string]string{
					"Node":      "nid{04d}",
					"HSNSwitch": "{id}",
				},
			},
			// expect the final expanded hostname
			wants: []string{
				"nid0001",
				"s100",
			},
		},
		{
			name: "policy_with_default_fallback_pattern",
			policy: Policy{
				DefaultPattern: "nid{04d}",
				ByType: map[string]string{
					"HSNSwitch": "switch-{id}",
				},
			},
			wants: []string{
				"nid0001",
			},
		},
		{
			name: "policy_with_no_patterns",
			policy: Policy{
				DefaultPattern: "",
				ByType:         map[string]string{},
			},
			wants: []string{""},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var results []string
			results[0], _ = tt.policy.HostnameFor("Node", 1, "")
			results[1], _ = tt.policy.HostnameFor("HSNSwitch", 100, "s100")

			for i := range results {
				if results[i] != tt.wants[i] {
					t.Errorf("expected %s but got %s instead for result %d",
						tt.wants[i], results[i], i)
				}
			}
		})
	}
}

// TODO: Implement TestSubnetPolicy_* for SubnetPolicy.

// func TestSubnetPolicyHostnameFor(t *testing.T) {
// 	tests := []struct {
// 		name   string
// 		subnet SubnetPolicy
// 		policy Policy
// 		want   string
// 	}{
// 		{
// 			name: "policy_use_subnet_rule",
// 			subnet: SubnetPolicy{
// 				Subnet: &net.IPNet{},
// 				Policy: Policy{
// 					DefaultPattern: "",
// 					ByType:         map[string]string{},
// 				},
// 			},
// 			policy: Policy{
// 				DefaultPattern: "",
// 				ByType:         map[string]string{},
// 			},
// 			want: "",
// 		},
// 	}

// 	for _, tt := range tests {
// 		tt := tt
// 		t.Run(tt.name, func(t *testing.T) {

// 			// get all masked IP addresses for CIDR
// 			ips, err := GetAllIPs("192.168.1.0/30")
// 			if err != nil {
// 				panic(err)
// 			}

// 			for _, ip := range ips {

// 				tt.policy.HostnameFor()
// 			}

// 			got := ""
// 			if got != tt.want {
// 				t.Errorf("")
// 			}
// 		})
// 	}
// }

// inc increments an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// GetAllIPs returns all IP addresses in a CIDR block
func GetAllIPs(cidr string) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string

	// Make a copy to avoid modifying original IP
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	return ips, nil
}
