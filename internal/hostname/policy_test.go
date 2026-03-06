package hostname

import (
	"net"
	"testing"
)

func TestPolicyHostnameFor(t *testing.T) {
	type call struct {
		compType  string
		nid       int64
		id        string
		want      string
		nameFound bool
	}
	tests := []struct {
		name   string
		policy Policy
		wants  []call
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
			wants: []call{
				{compType: "Node", nid: 1, id: "", want: "nid0001", nameFound: true},
				{compType: "HSNSwitch", nid: 100, id: "s100", want: "s100", nameFound: true},
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
			wants: []call{
				{compType: "Node", nid: 1, id: "", want: "nid0001", nameFound: true},
				{compType: "HSNSwitch", nid: 100, id: "s100", want: "switch-s100", nameFound: true},
			},
		},
		{
			name: "policy_with_no_patterns",
			policy: Policy{
				DefaultPattern: "",
				ByType:         map[string]string{},
			},
			wants: []call{
				{compType: "Node", nid: 1, id: "", want: "", nameFound: false},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.wants {
				tc := tc
				t.Run(tc.compType, func(t *testing.T) {
					got, nameFound := tt.policy.HostnameFor(tc.compType, tc.nid, tc.id)
					if nameFound != tc.nameFound {
						t.Fatalf("HostnameFor(%q, %d, %q): nameFound was %v, expected %v", tc.compType, tc.nid, tc.id, nameFound, tc.nameFound)
					}
					if got != tc.want {
						t.Fatalf("HostnameFor(%q, %d, %q) = %q, want %q", tc.compType, tc.nid, tc.id, got, tc.want)
					}
				})
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
