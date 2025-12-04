package coresmd

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the configuration for the coresmd plugin
type Config struct {
	// SMD settings
	SMD struct {
		BaseURL       string `yaml:"base_url"`
		CACertPath    string `yaml:"ca_cert_path"`
		CacheDuration string `yaml:"cache_duration"`
	} `yaml:"smd"`

	// Boot settings
	Boot struct {
		ScriptBaseURL string `yaml:"script_base_url"`
	} `yaml:"boot"`

	// DHCP settings
	DHCP struct {
		LeaseDuration string `yaml:"lease_duration"`
	} `yaml:"dhcp"`

	// TFTP settings
	TFTP struct {
		SinglePort bool `yaml:"single_port"`
	} `yaml:"tftp"`

	// Hostname settings
	Hostname struct {
		NodePattern string `yaml:"node_pattern"`
		BMCPattern  string `yaml:"bmc_pattern"`
		Domain      string `yaml:"domain"`
	} `yaml:"hostname"`
}

// LoadConfig reads and parses the configuration file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if not specified
	if config.SMD.CacheDuration == "" {
		config.SMD.CacheDuration = "30s"
	}
	if config.DHCP.LeaseDuration == "" {
		config.DHCP.LeaseDuration = "1h"
	}
	if config.Hostname.NodePattern == "" {
		config.Hostname.NodePattern = "nid{04d}"
	}

	return &config, nil
}

// Validate checks that required configuration values are set
func (c *Config) Validate() error {
	if c.SMD.BaseURL == "" {
		return fmt.Errorf("smd.base_url is required")
	}
	if c.Boot.ScriptBaseURL == "" {
		return fmt.Errorf("boot.script_base_url is required")
	}

	// Validate durations
	if _, err := time.ParseDuration(c.SMD.CacheDuration); err != nil {
		return fmt.Errorf("invalid smd.cache_duration: %w", err)
	}
	if _, err := time.ParseDuration(c.DHCP.LeaseDuration); err != nil {
		return fmt.Errorf("invalid dhcp.lease_duration: %w", err)
	}

	return nil
}
