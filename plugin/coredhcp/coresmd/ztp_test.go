// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package coresmd

import (
	"net"
	"strings"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/openchami/coresmd/internal/cache"
	"github.com/openchami/coresmd/internal/smdclient"
)

const testZTPURL = "https://ztp.example.test:8443/ztp.json?site=lanl&version=1"

func TestParseConfigZTP(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"mapping", []string{"ztp_user_class=NVOS-ZTP," + testZTPURL}, false},
		{"URL separators preserved", []string{"ztp_user_class=NVOS-ZTP,https://ztp.example.test/a,b?site=x=y"}, false},
		{"multiple mappings", []string{"ztp_user_class=NVOS-ZTP," + testZTPURL, "ztp_user_class=OTHER-ZTP,https://ztp.example.test/other"}, false},
		{"missing separator", []string{"ztp_user_class=NVOS-ZTP"}, true},
		{"empty class", []string{"ztp_user_class=," + testZTPURL}, true},
		{"whitespace in class", []string{"ztp_user_class=NVOS ZTP," + testZTPURL}, true},
		{"control byte in class", []string{"ztp_user_class=NVOS\x00ZTP," + testZTPURL}, true},
		{"oversized class", []string{"ztp_user_class=" + strings.Repeat("x", 256) + "," + testZTPURL}, true},
		{"empty URL", []string{"ztp_user_class=NVOS-ZTP,"}, true},
		{"relative URL", []string{"ztp_user_class=NVOS-ZTP,/ztp.json"}, true},
		{"missing host", []string{"ztp_user_class=NVOS-ZTP,https:///ztp.json"}, true},
		{"invalid URL", []string{"ztp_user_class=NVOS-ZTP,http://["}, true},
		{"duplicate class", []string{"ztp_user_class=NVOS-ZTP," + testZTPURL, "ztp_user_class=NVOS-ZTP,https://ztp.example.test/other"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, errs := parseConfig(tt.args...)
			if (len(errs) != 0) != tt.wantErr {
				t.Fatalf("parseConfig errors=%v, wantErr=%v", errs, tt.wantErr)
			}
			if !tt.wantErr {
				for _, arg := range tt.args {
					if !strings.Contains(cfg.String(), arg) {
						t.Errorf("Config.String() missing %q", arg)
					}
				}
			}
		})
	}
}

func TestConfigStringZTPRedactsPassword(t *testing.T) {
	cfg, errs := parseConfig("ztp_user_class=NVOS-ZTP,scp://ztp:secret-password@ztp.example.test/ztp.json")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if strings.Contains(cfg.String(), "secret-password") {
		t.Fatal("Config.String exposes the provisioning URL password")
	}
}

func TestHandler4ZTPBootSelection(t *testing.T) {
	// Handler4 uses package globals; restore them and keep these cases sequential.
	oldConfig, oldCache := globalConfig, smdCache
	t.Cleanup(func() { globalConfig, smdCache = oldConfig, oldCache })
	const mac = "02:00:00:00:00:01"
	const address = "192.0.2.10"
	const ipxeURL = "https://boot.example.test/bootscript?mac=" + mac
	for _, tt := range []struct {
		name     string
		class    []byte
		mapping  bool
		unknown  bool
		ignore   bool
		wantBoot string
		wantZTP  bool
	}{
		{"NVOS raw", []byte("NVOS-ZTP"), true, false, false, testZTPURL, true},
		{"NVOS RFC3004", dhcpv4.Strings{"NVOS-ZTP"}.ToBytes(), true, false, false, testZTPURL, true},
		{"multiple classes", dhcpv4.Strings{"other", "NVOS-ZTP"}.ToBytes(), true, false, false, testZTPURL, true},
		{"first configured mapping wins", dhcpv4.Strings{"OTHER-ZTP", "NVOS-ZTP"}.ToBytes(), true, false, false, testZTPURL, true},
		{"other mapped class", dhcpv4.Strings{"OTHER-ZTP"}.ToBytes(), true, false, false, "https://ztp.example.test/other", true},
		{"no mapping", []byte("NVOS-ZTP"), false, false, false, "ipxe-x86_64.efi", false},
		{"missing class", nil, true, false, false, "ipxe-x86_64.efi", false},
		{"unknown class", []byte("unknown"), true, false, false, "ipxe-x86_64.efi", false},
		{"exact match only", []byte("NVOS-ZTP-extra"), true, false, false, "ipxe-x86_64.efi", false},
		{"case sensitive", []byte("nvos-ztp"), true, false, false, "ipxe-x86_64.efi", false},
		{"malformed RFC3004", append(dhcpv4.Strings{"NVOS-ZTP"}.ToBytes(), 3, 'x'), true, false, false, "ipxe-x86_64.efi", false},
		{"raw iPXE", []byte("iPXE"), true, false, false, ipxeURL, false},
		{"RFC3004 iPXE", dhcpv4.Strings{"iPXE"}.ToBytes(), true, false, false, ipxeURL, false},
		{"unknown MAC passes through", []byte("NVOS-ZTP"), true, true, false, "previous.efi", false},
		{"ignore rule still drops", []byte("NVOS-ZTP"), true, false, true, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"svc_base_uri=https://smd.example.test", "ipxe_uri=https://boot.example.test/bootscript", "lease_time=1h", "domain=example.test", "rule=hostname:test-host,routers:192.0.2.1,cidr:24"}
			if tt.mapping {
				args = append(args, "ztp_user_class=NVOS-ZTP,"+testZTPURL, "ztp_user_class=OTHER-ZTP,https://ztp.example.test/other")
			}
			if tt.ignore {
				args = append([]string{"rule=ignore:true"}, args...)
			}
			cfg, errs := parseConfig(args...)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			if _, errs := cfg.validate(); len(errs) != 0 {
				t.Fatal(errs)
			}
			globalConfig = cfg
			smdCache = &cache.Cache{
				EthernetInterfaces: map[string]smdclient.EthernetInterface{},
				Components:         map[string]smdclient.Component{"x0c0s0b0n0": {ID: "x0c0s0b0n0", Type: "Node", NID: 1}},
			}
			if !tt.unknown {
				smdCache.EthernetInterfaces[mac] = smdclient.EthernetInterface{MACAddress: mac, ComponentID: "x0c0s0b0n0", IPAddresses: []smdclient.IPAddress{{IPAddress: address}}}
			}
			for _, message := range []struct {
				name              string
				request, response dhcpv4.MessageType
			}{
				{"discover", dhcpv4.MessageTypeDiscover, dhcpv4.MessageTypeOffer},
				{"request", dhcpv4.MessageTypeRequest, dhcpv4.MessageTypeAck},
			} {
				t.Run(message.name, func(t *testing.T) {
					hw, err := net.ParseMAC(mac)
					if err != nil {
						t.Fatal(err)
					}
					req, err := dhcpv4.New(dhcpv4.WithHwAddr(hw), dhcpv4.WithMessageType(message.request))
					if err != nil {
						t.Fatal(err)
					}
					req.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64))
					if tt.class != nil {
						req.Options[dhcpv4.OptionUserClassInformation.Code()] = tt.class
					}
					resp, err := dhcpv4.NewReplyFromRequest(req, dhcpv4.WithMessageType(message.response))
					if err != nil {
						t.Fatal(err)
					}
					resp.UpdateOption(dhcpv4.OptBootFileName("previous.efi"))
					resp.UpdateOption(dhcpv4.OptTFTPServerName("old-tftp.example.test"))
					got, stop := Handler4(req, resp)
					if tt.ignore {
						if got != nil || !stop {
							t.Fatal("ignore rule did not drop request")
						}
						return
					}
					if got == nil {
						t.Fatal("unexpected dropped response")
					}
					if stop == tt.unknown {
						t.Errorf("stop=%v, unknown=%v", stop, tt.unknown)
					}
					if boot := string(got.Options.Get(dhcpv4.OptionBootfileName)); boot != tt.wantBoot {
						t.Errorf("boot URL=%q, want %q", boot, tt.wantBoot)
					}
					if tt.unknown {
						return
					}
					if !got.YourIPAddr.Equal(net.ParseIP(address)) {
						t.Errorf("address=%s", got.YourIPAddr)
					}
					for _, option := range []dhcpv4.OptionCode{dhcpv4.OptionIPAddressLeaseTime, dhcpv4.OptionHostName, dhcpv4.OptionRouter, dhcpv4.OptionSubnetMask} {
						if !got.Options.Has(option) {
							t.Errorf("missing network option %v", option)
						}
					}
					if got.MessageType() != message.response {
						t.Errorf("message type=%v", got.MessageType())
					}
					if tt.wantZTP && got.Options.Has(dhcpv4.OptionTFTPServerName) {
						t.Error("ZTP response retains conflicting TFTP server option")
					}
				})
			}
		})
	}
}
