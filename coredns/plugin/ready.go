package plugin

import (
	"time"
)

// Ready checks if the plugin's cache is initialized, has been updated at least once (i.e., LastUpdated is non-zero), and is not older than 5 minutes.
// It implements the ready.Readiness interface (Ready() bool) for https://coredns.io/plugins/ready/.
// Returns true if the cache is ready, otherwise false.
func (p Plugin) Ready() bool {
	if p.cache == nil {
		return false
	}

	// Cache is ready if it has been updated at least once
	if p.cache.LastUpdated.IsZero() {
		return false
	}

	// Cache is ready if it's not too old (e.g., less than 5 minutes)
	if time.Since(p.cache.LastUpdated) > 5*time.Minute {
		return false
	}

	return true
}

func (p Plugin) Health() bool {
	return p.Ready()
}

func (p Plugin) OnStartupComplete() error {
	// Plugin startup is complete
	return nil
}

func (p Plugin) OnShutdown() error {
	// Plugin shutdown is complete
	return nil
}
