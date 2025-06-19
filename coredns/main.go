package main

import (
	_ "github.com/OpenCHAMI/coresmd/coredns/plugin"

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
