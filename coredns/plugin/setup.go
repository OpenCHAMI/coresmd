package plugin

import (
	"fmt"
	"net/url"
	"time"

	"github.com/OpenCHAMI/coresmd/coresmd"
	"github.com/OpenCHAMI/coresmd/internal/version"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/sirupsen/logrus"
)

// Plugin represents the coresmd plugin
type Plugin struct {
	Next plugin.Handler

	// SMD connection settings
	smdURL        string
	caCert        string
	cacheDuration string

	// Zone configuration
	zones []Zone

	// Shared infrastructure
	cache     *coresmd.Cache
	smdClient *coresmd.SmdClient
}

// Global variables for shared cache across plugin instances
var (
	sharedCache     *coresmd.Cache
	sharedSmdClient *coresmd.SmdClient
	log             = logrus.NewEntry(logrus.New())
)

func init() {
	plugin.Register("coresmd", setup)
}

// setup is the function that gets called when the plugin is "setup" in Corefile
func setup(c *caddy.Controller) error {
	coresmd, err := parse(c)
	if err != nil {
		return plugin.Error("coresmd", err)
	}

	// Register the plugin
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		coresmd.Next = next
		return coresmd
	})

	// Register metrics and readiness hooks
	c.OnStartup(func() error {
		// Call plugin OnStartup for version logging and initialization
		if err := coresmd.OnStartup(); err != nil {
			return err
		}

		// Update cache metrics periodically
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if coresmd.cache != nil {
					coresmd.cache.Mutex.RLock()
					if !coresmd.cache.LastUpdated.IsZero() {
						age := time.Since(coresmd.cache.LastUpdated).Seconds()
						SMDCacheAge.WithLabelValues("default").Set(age)
						SMDCacheSize.WithLabelValues("default", "ethernet_interfaces").Set(float64(len(coresmd.cache.EthernetInterfaces)))
						SMDCacheSize.WithLabelValues("default", "components").Set(float64(len(coresmd.cache.Components)))
					}
					coresmd.cache.Mutex.RUnlock()
				}
			}
		}()
		return nil
	})

	return nil
}

// parse parses the Corefile configuration for the coresmd plugin
func parse(c *caddy.Controller) (*Plugin, error) {
	p := &Plugin{}

	for c.Next() {
		for c.NextBlock() {
			switch c.Val() {
			case "smd_url":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				p.smdURL = c.Val()
			case "ca_cert":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				p.caCert = c.Val()
			case "cache_duration":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				p.cacheDuration = c.Val()
			case "zone":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				zoneName := c.Val()
				zone, err := parseZone(c, zoneName)
				if err != nil {
					return nil, err
				}
				p.zones = append(p.zones, zone)
			default:
				return nil, c.Errf("unknown directive '%s'", c.Val())
			}
		}
	}

	// Validate required configuration
	if p.smdURL == "" {
		return nil, fmt.Errorf("smd_url is required")
	}
	if p.cacheDuration == "" {
		p.cacheDuration = "30s" // Default cache duration
	}

	return p, nil
}

// parseZone parses zone configuration blocks
func parseZone(c *caddy.Controller, zoneName string) (Zone, error) {
	zone := Zone{Name: zoneName}

	for c.NextBlock() {
		switch c.Val() {
		case "nodes":
			if !c.NextArg() {
				return zone, c.ArgErr()
			}
			zone.NodePattern = c.Val()
		case "bmcs":
			if !c.NextArg() {
				return zone, c.ArgErr()
			}
			zone.BMCPattern = c.Val()
		default:
			return zone, c.Errf("unknown zone directive '%s'", c.Val())
		}
	}

	return zone, nil
}

// OnStartup is called when the plugin starts up
func (p *Plugin) OnStartup() error {
	// Log version information
	log.Infof("initializing coresmd/coredns %s (%s), built %s",
		version.Version, version.GitState, version.BuildTime)
	log.WithFields(version.VersionInfo).Debugln("detailed version info")

	// Initialize shared cache if not already done
	if sharedCache == nil {
		baseURL, err := url.Parse(p.smdURL)
		if err != nil {
			return fmt.Errorf("failed to parse SMD URL: %w", err)
		}

		sharedSmdClient = coresmd.NewSmdClient(baseURL)

		// Set up CA certificate if provided
		if p.caCert != "" {
			if err := sharedSmdClient.UseCACert(p.caCert); err != nil {
				return fmt.Errorf("failed to set CA certificate: %w", err)
			}
			log.Infof("set CA certificate for SMD to the contents of %s", p.caCert)
		} else {
			log.Infof("CA certificate path was empty, not setting")
		}

		// Create cache
		sharedCache, err = coresmd.NewCache(p.cacheDuration, sharedSmdClient)
		if err != nil {
			return fmt.Errorf("failed to create cache: %w", err)
		}

		// Start cache refresh loop
		sharedCache.RefreshLoop()

		log.Infof("coresmd cache initialized with base URL %s and validity duration %s",
			sharedSmdClient.BaseURL, sharedCache.Duration.String())
	}

	// Assign shared resources to this plugin instance
	p.cache = sharedCache
	p.smdClient = sharedSmdClient

	// Set default zones if none configured
	if len(p.zones) == 0 {
		p.zones = []Zone{
			{
				Name:        "cluster.local",
				NodePattern: "nid{04d}",
				BMCPattern:  "bmc-{id}",
			},
		}
	}

	return nil
}
