package debug

import (
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/sirupsen/logrus"
)

func DebugRequest(log *logrus.Entry, req *dhcpv4.DHCPv4) {
	log.Debugf("REQUEST: %v", req.Summary())
}

func DebugResponse(log *logrus.Entry, resp *dhcpv4.DHCPv4) {
	log.Debugf("RESPONSE: %v", resp.Summary())
}
