package coredns

import (
	"time"
)

// Ready implements the ready.Readiness interface.
func (p Plugin) Ready() bool {
	if p.cache == nil {
		return false
	}

	// Check if cache has been initialized
	p.cache.Mutex.RLock()
	defer p.cache.Mutex.RUnlock()

	// Cache is ready if it has been updated at least once
	if p.cache.LastUpdated.IsZero() {
		return false
	}

	// Cache is ready if it has some data
	if len(p.cache.EthernetInterfaces) == 0 && len(p.cache.Components) == 0 {
		return false
	}

	// Cache is ready if it's not too old (e.g., less than 5 minutes)
	if time.Since(p.cache.LastUpdated) > 5*time.Minute {
		return false
	}

	return true
}

// Health implements a simple health check
func (p Plugin) Health() bool {
	return p.Ready()
}

// OnStartupComplete is called when the plugin startup is complete
func (p Plugin) OnStartupComplete() error {
	// Plugin startup is complete
	return nil
}

// OnShutdown is called when the plugin is shutting down
func (p Plugin) OnShutdown() error {
	// Plugin shutdown is complete
	return nil
}
