package main

import (
	_ "github.com/openchami/coresmd/coredns/plugin"

	// CoreDNS plugins activated below
	_ "github.com/coredns/coredns/plugin/bind"
	_ "github.com/coredns/coredns/plugin/cache"
	_ "github.com/coredns/coredns/plugin/debug"
	_ "github.com/coredns/coredns/plugin/errors"
	_ "github.com/coredns/coredns/plugin/file"
	_ "github.com/coredns/coredns/plugin/forward"
	_ "github.com/coredns/coredns/plugin/health"
	_ "github.com/coredns/coredns/plugin/hosts"
	_ "github.com/coredns/coredns/plugin/loadbalance"
	_ "github.com/coredns/coredns/plugin/log"
	_ "github.com/coredns/coredns/plugin/loop"
	_ "github.com/coredns/coredns/plugin/metrics" // ← prometheus metrics
	_ "github.com/coredns/coredns/plugin/ready"
	_ "github.com/coredns/coredns/plugin/reload"
	_ "github.com/coredns/coredns/plugin/root"
	_ "github.com/coredns/coredns/plugin/template"
	_ "github.com/coredns/coredns/plugin/whoami"
	_ "github.com/ori-edge/k8s_gateway"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

// List of plugins in the desired order. Add or remove plugins as needed.
var directives = []string{
	// Standard plugins (add/remove as needed)
	"errors",
	"log",
	"health",
	"ready",
	"prometheus",
	// Your plugin
	"coresmd",
	"k8s_gateway",
	// Standard plugins (add/remove as needed)
	"forward",
	"cache",
	"reload",
	"loadbalance",
	"loop",
	"bind",
	"debug",
	"template",
	"whoami",
	"hosts",
	"file",
	"root",
	"startup",
	"shutdown",
}

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
