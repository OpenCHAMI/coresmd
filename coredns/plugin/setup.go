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

	// The outer for c.Next() handles each "coresmd" stanza in the Corefile.
	// Typically you'd have only one, but this loop allows multiple if needed.
	for c.Next() {
		// The inner for c.NextBlock() loops through the directives
		// that appear inside the "coresmd { ... }" block.
		for c.NextBlock() {
			directive := c.Val()
			log.Debugf("Parsing directive: %s", directive)

			switch directive {

			case "smd_url":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				// Optional: Return an error if smd_url was already set
				if p.smdURL != "" {
					return nil, c.Errf("smd_url already specified")
				}
				p.smdURL = c.Val()
				log.Debugf("Set smd_url to: %s", p.smdURL)

			case "ca_cert":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				p.caCert = c.Val()
				log.Debugf("Set ca_cert to: %s", p.caCert)

			case "cache_duration":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				p.cacheDuration = c.Val()
				log.Debugf("Set cache_duration to: %s", p.cacheDuration)

			case "zone":
				// Example usage in Corefile:
				//   zone cluster.local {
				//       nodes nid{04d}
				//       bmcs bmc-{id}
				//   }
				//
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				zoneName := c.Val()
				log.Debugf("Parsing zone: %s", zoneName)
				zone, err := parseZone(c, zoneName)
				if err != nil {
					return nil, err
				}
				p.zones = append(p.zones, zone)
				log.Debugf("Added zone: %+v", zone)

			default:
				return nil, c.Errf("unknown directive '%s'", directive)
			}
		}
	}

	// Final validation & defaults
	if p.smdURL == "" {
		return nil, fmt.Errorf("smd_url is required")
	}
	if p.cacheDuration == "" {
		p.cacheDuration = "30s"
	}

	return p, nil
}

// parseZone reads the lines inside a "zone <name> { ... }" block.
func parseZone(c *caddy.Controller, zoneName string) (Zone, error) {
	zone := Zone{Name: zoneName}

	// Enter the block for the zone directive (consume the opening brace if present)
	if !c.Next() {
		return zone, c.Errf("expected opening brace or directive after zone name")
	}
	if c.Val() != "{" {
		return zone, c.Errf("expected '{' after zone name, got '%s'", c.Val())
	}

	// Read the directives inside the zone block
	for c.Next() {
		directive := c.Val()
		if directive == "}" || directive == "zone" {
			// End of this zone block or start of a new zone
			break
		}
		switch directive {
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
			return zone, c.Errf("unknown zone directive '%s'", directive)
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
	} else {
		log.Infof("configured zones: %v", p.zones)
	}
	// Log cache initialization
	log.Infof("coresmd cache initialized with %d zones", len(p.zones))

	return nil
}
