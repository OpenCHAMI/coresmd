package plugin

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openchami/coresmd/coresmd"
)

func TestMetricsRegistration(t *testing.T) {
	// Test that metrics are properly registered
	metrics := []prometheus.Collector{
		RequestCount,
		RequestDuration,
		CacheHits,
		CacheMisses,
		SMDCacheAge,
		SMDCacheSize,
	}

	for _, metric := range metrics {
		if metric == nil {
			t.Errorf("Metric is nil")
		}
	}
}

func TestReadyFunction(t *testing.T) {
	// Test Ready function with nil cache
	p := &Plugin{cache: nil}
	if p.Ready() {
		t.Error("Expected Ready() to return false for nil cache")
	}

	// Test Ready function with empty cache
	p.cache = &coresmd.Cache{
		Duration:    1 * time.Minute,
		LastUpdated: time.Time{}, // Zero time
		Mutex:       sync.RWMutex{},
	}
	if p.Ready() {
		t.Error("Expected Ready() to return false for cache with zero LastUpdated")
	}

	// Test Ready function with valid cache
	p.cache.LastUpdated = time.Now()
	p.cache.EthernetInterfaces = map[string]coresmd.EthernetInterface{
		"test": {
			MACAddress:  "00:11:22:33:44:55",
			ComponentID: "test001",
			Type:        "Node",
		},
	}
	if !p.Ready() {
		t.Error("Expected Ready() to return true for valid cache")
	}

	// Test Ready function with old cache
	p.cache.LastUpdated = time.Now().Add(-10 * time.Minute)
	if p.Ready() {
		t.Error("Expected Ready() to return false for old cache")
	}
}

func TestHealthFunction(t *testing.T) {
	p := &Plugin{cache: nil}

	// Health should match Ready
	if p.Health() != p.Ready() {
		t.Error("Health() should return the same value as Ready()")
	}
}
