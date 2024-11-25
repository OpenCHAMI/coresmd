package ipxe

import (
	"encoding/binary"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/sirupsen/logrus"
)

func ServeIPXEBootloader(l *logrus.Entry, req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.Options.Has(dhcpv4.OptionClientSystemArchitectureType) {
		var carch iana.Arch
		carchBytes := req.Options.Get(dhcpv4.OptionClientSystemArchitectureType)
		l.Debugf("client architecture of %s is %v (%q)", req.ClientHWAddr, carchBytes, string(carchBytes))
		carch = iana.Arch(binary.BigEndian.Uint16(carchBytes))
		switch carch {
		case iana.INTEL_X86PC:
			// iPXE legacy 32-bit x86 bootloader
			resp.Options.Update(dhcpv4.OptBootFileName("undionly.kpxe"))
			return resp, true
		case iana.EFI_IA32:
			// iPXE EFI 32-bit bootloader
			resp.Options.Update(dhcpv4.OptBootFileName("ipxe-i386.efi"))
			return resp, true
		case iana.EFI_X86_64:
			// iPXE 64-bit x86 bootloader
			resp.Options.Update(dhcpv4.OptBootFileName("ipxe-x86_64.efi"))
			return resp, true
		case iana.EFI_ARM32:
			// iPXE EFI 32-bit ARM bootloader
			resp.Options.Update(dhcpv4.OptBootFileName("ipxe-arm32.efi"))
			return resp, true
		case iana.EFI_ARM64:
			// iPXE EFI 64-bit ARM bootloader
			resp.Options.Update(dhcpv4.OptBootFileName("ipxe-arm64.efi"))
			return resp, true
		default:
			l.Errorf("no iPXE bootloader available for unknown architecture: %d (%s)", carch, carch.String())
			return resp, false
		}
	} else {
		l.Errorf("client did not present an architecture, unable to provide correct iPXE bootloader")
		return resp, false
	}
}
