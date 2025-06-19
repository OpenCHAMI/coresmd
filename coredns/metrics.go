package coredns

import (
	"github.com/coredns/coredns/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestCount is the total number of DNS requests processed by the coresmd plugin
	RequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "coresmd",
		Name:      "requests_total",
		Help:      "Counter of DNS requests made to the coresmd plugin.",
	}, []string{"server", "zone", "type"})

	// RequestDuration is the time taken to process DNS requests
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: "coresmd",
		Name:      "request_duration_seconds",
		Buckets:   plugin.TimeBuckets,
		Help:      "Histogram of the time (in seconds) each request took.",
	}, []string{"server", "zone"})

	// CacheHits is the number of successful cache lookups
	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "coresmd",
		Name:      "cache_hits_total",
		Help:      "Counter of successful cache lookups in the coresmd plugin.",
	}, []string{"server", "zone", "record_type"})

	// CacheMisses is the number of failed cache lookups
	CacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "coresmd",
		Name:      "cache_misses_total",
		Help:      "Counter of failed cache lookups in the coresmd plugin.",
	}, []string{"server", "zone", "record_type"})

	// SMDCacheAge is the age of the SMD cache in seconds
	SMDCacheAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "coresmd",
		Name:      "smd_cache_age_seconds",
		Help:      "Age of the SMD cache in seconds.",
	}, []string{"server"})

	// SMDCacheSize is the number of entries in the SMD cache
	SMDCacheSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "coresmd",
		Name:      "smd_cache_size",
		Help:      "Number of entries in the SMD cache.",
	}, []string{"server", "type"})
)
