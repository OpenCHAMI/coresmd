// SPDX-FileCopyrightText: Copyright 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package iface

import (
	"net"
	"testing"

	"github.com/openchami/coresmd/internal/cache"
	"github.com/openchami/coresmd/internal/smdclient"
	"github.com/openchami/coresmd/internal/subnet"
)

func TestLookupMACWithSubnet_Table(t *testing.T) {
	mkCache := func() *cache.Cache {
		c := &cache.Cache{}
		c.EthernetInterfaces = map[string]smdclient.EthernetInterface{
			"aa:bb:cc:dd:ee:ff": {
				MACAddress:  "aa:bb:cc:dd:ee:ff",
				ComponentID: "x0c0s0b0n0",
				IPAddresses: []smdclient.IPAddress{
					{IPAddress: "10.40.1.10"},
					{IPAddress: "10.40.3.20"},
				},
			},
			"11:22:33:44:55:66": {
				MACAddress:  "11:22:33:44:55:66",
				ComponentID: "x0c0s0b0b0",
				IPAddresses: []smdclient.IPAddress{
					{IPAddress: "10.40.1.50"},
				},
			},
			"00:00:00:00:00:01": {
				MACAddress:  "00:00:00:00:00:01",
				ComponentID: "x0c0s1b0n0",
				IPAddresses: []smdclient.IPAddress{},
			},
		}
		c.Components = map[string]smdclient.Component{
			"x0c0s0b0n0": {ID: "x0c0s0b0n0", NID: 7, Type: "Node"},
			"x0c0s0b0b0": {ID: "x0c0s0b0b0", Type: "NodeBMC"},
			"x0c0s1b0n0": {ID: "x0c0s1b0n0", NID: 8, Type: "Node"},
		}
		return c
	}

	mkSubnetCtx := func() *subnet.SubnetContext {
		sc := subnet.NewSubnetContext()
		sc.AddSubnet("10.40.1.0/24", "10.40.1.1")
		sc.AddSubnet("10.40.3.0/24", "10.40.3.1")
		return sc
	}

	tests := []struct {
		name    string
		mac     string
		giaddr  net.IP
		cache   *cache.Cache
		sc      *subnet.SubnetContext
		wantErr bool
		check   func(t *testing.T, ii IfaceInfo)
	}{
		{
			name:    "nil_cache",
			mac:     "aa:bb:cc:dd:ee:ff",
			giaddr:  net.ParseIP("10.40.1.1"),
			cache:   nil,
			sc:      mkSubnetCtx(),
			wantErr: true,
		},
		{
			name:    "mac_not_found",
			mac:     "ff:ff:ff:ff:ff:ff",
			giaddr:  net.ParseIP("10.40.1.1"),
			cache:   mkCache(),
			sc:      mkSubnetCtx(),
			wantErr: true,
		},
		{
			name:   "giaddr_filters_to_subnet1",
			mac:    "aa:bb:cc:dd:ee:ff",
			giaddr: net.ParseIP("10.40.1.1"),
			cache:  mkCache(),
			sc:     mkSubnetCtx(),
			check: func(t *testing.T, ii IfaceInfo) {
				if len(ii.IPList) != 1 {
					t.Fatalf("expected 1 IP, got %d: %v", len(ii.IPList), ii.IPList)
				}
				if !ii.IPList[0].Equal(net.ParseIP("10.40.1.10")) {
					t.Fatalf("expected 10.40.1.10, got %s", ii.IPList[0])
				}
			},
		},
		{
			name:   "giaddr_filters_to_subnet2",
			mac:    "aa:bb:cc:dd:ee:ff",
			giaddr: net.ParseIP("10.40.3.1"),
			cache:  mkCache(),
			sc:     mkSubnetCtx(),
			check: func(t *testing.T, ii IfaceInfo) {
				if len(ii.IPList) != 1 {
					t.Fatalf("expected 1 IP, got %d: %v", len(ii.IPList), ii.IPList)
				}
				if !ii.IPList[0].Equal(net.ParseIP("10.40.3.20")) {
					t.Fatalf("expected 10.40.3.20, got %s", ii.IPList[0])
				}
			},
		},
		{
			name:   "giaddr_not_in_any_subnet_returns_all_ips",
			mac:    "aa:bb:cc:dd:ee:ff",
			giaddr: net.ParseIP("192.168.1.1"),
			cache:  mkCache(),
			sc:     mkSubnetCtx(),
			check: func(t *testing.T, ii IfaceInfo) {
				if len(ii.IPList) != 2 {
					t.Fatalf("expected 2 IPs (fallback), got %d: %v", len(ii.IPList), ii.IPList)
				}
			},
		},
		{
			name:   "no_giaddr_returns_all_ips",
			mac:    "aa:bb:cc:dd:ee:ff",
			giaddr: net.IPv4zero,
			cache:  mkCache(),
			sc:     mkSubnetCtx(),
			check: func(t *testing.T, ii IfaceInfo) {
				if len(ii.IPList) != 2 {
					t.Fatalf("expected 2 IPs, got %d: %v", len(ii.IPList), ii.IPList)
				}
			},
		},
		{
			name:   "nil_subnet_context_returns_all_ips",
			mac:    "aa:bb:cc:dd:ee:ff",
			giaddr: net.ParseIP("10.40.1.1"),
			cache:  mkCache(),
			sc:     nil,
			check: func(t *testing.T, ii IfaceInfo) {
				if len(ii.IPList) != 2 {
					t.Fatalf("expected 2 IPs, got %d: %v", len(ii.IPList), ii.IPList)
				}
			},
		},
		{
			name:    "no_ips_matching_subnet_returns_error",
			mac:     "11:22:33:44:55:66",
			giaddr:  net.ParseIP("10.40.3.1"),
			cache:   mkCache(),
			sc:      mkSubnetCtx(),
			wantErr: true,
		},
		{
			name:    "no_ips_at_all_returns_error",
			mac:     "00:00:00:00:00:01",
			giaddr:  net.ParseIP("10.40.1.1"),
			cache:   mkCache(),
			sc:      mkSubnetCtx(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii, err := LookupMACWithSubnet(nil, tt.mac, tt.giaddr, tt.cache, tt.sc)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.check != nil {
				tt.check(t, ii)
			}
		})
	}
}

func TestLookupMAC_Table(t *testing.T) {
	mkCache := func() *cache.Cache {
		c := &cache.Cache{}
		c.EthernetInterfaces = map[string]smdclient.EthernetInterface{
			"aa:bb:cc:dd:ee:ff": {MACAddress: "aa:bb:cc:dd:ee:ff", ComponentID: "x0c0s0b0n0", IPAddresses: []smdclient.IPAddress{{IPAddress: "172.16.0.10"}, {IPAddress: "2001:db8::1"}}},
			"11:22:33:44:55:66": {MACAddress: "11:22:33:44:55:66", ComponentID: "x0c0s0b0b0", IPAddresses: []smdclient.IPAddress{{IPAddress: "172.16.10.20"}}},
		}
		c.Components = map[string]smdclient.Component{
			"x0c0s0b0n0": {ID: "x0c0s0b0n0", NID: 7, Type: "Node"},
			"x0c0s0b0b0": {ID: "x0c0s0b0b0", Type: "NodeBMC"},
		}
		return c
	}

	tests := []struct {
		name    string
		mac     string
		cache   *cache.Cache
		wantErr bool
		check   func(t *testing.T, ii IfaceInfo)
	}{
		{name: "nil_cache", mac: "aa:bb:cc:dd:ee:ff", cache: nil, wantErr: true},
		{name: "mac_not_found", mac: "00:00:00:00:00:00", cache: mkCache(), wantErr: true},
		{name: "component_not_found", mac: "aa:bb:cc:dd:ee:ff", cache: func() *cache.Cache { c := mkCache(); delete(c.Components, "x0c0s0b0n0"); return c }(), wantErr: true},
		{
			name:  "node_success_populates_nid_and_ips",
			mac:   "aa:bb:cc:dd:ee:ff",
			cache: mkCache(),
			check: func(t *testing.T, ii IfaceInfo) {
				if ii.Type != "Node" || ii.CompID != "x0c0s0b0n0" {
					t.Fatalf("type/id=%s/%s", ii.Type, ii.CompID)
				}
				if ii.CompNID != 7 {
					t.Fatalf("nid=%d", ii.CompNID)
				}
				if len(ii.IPList) != 2 || !ii.IPList[0].Equal(net.ParseIP("172.16.0.10")) {
					t.Fatalf("ips=%v", ii.IPList)
				}
			},
		},
		{
			name:  "bmc_success_does_not_set_nid",
			mac:   "11:22:33:44:55:66",
			cache: mkCache(),
			check: func(t *testing.T, ii IfaceInfo) {
				if ii.Type != "NodeBMC" {
					t.Fatalf("type=%s", ii.Type)
				}
				if ii.CompNID != 0 {
					t.Fatalf("nid=%d", ii.CompNID)
				}
			},
		},
		{
			name: "no_ips_returns_error",
			mac:  "aa:bb:cc:dd:ee:ff",
			cache: func() *cache.Cache {
				c := mkCache()
				ei := c.EthernetInterfaces["aa:bb:cc:dd:ee:ff"]
				ei.IPAddresses = nil
				c.EthernetInterfaces["aa:bb:cc:dd:ee:ff"] = ei
				return c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ii, err := LookupMAC(nil, tt.mac, tt.cache)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.check != nil {
				tt.check(t, ii)
			}
		})
	}
}
