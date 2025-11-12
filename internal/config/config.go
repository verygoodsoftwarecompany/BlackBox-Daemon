// Package config provides configuration management for the BlackBox daemon.
// It supports environment-based configuration with validation and sensible defaults.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/emitter"
)

// Config holds all configuration parameters for the BlackBox daemon.
// Configuration is loaded from environment variables with fallback to defaults.
type Config struct {
	// Buffer configuration - controls telemetry retention
	// BufferWindowSize determines how long telemetry is kept in the ring buffer
	BufferWindowSize time.Duration `json:"buffer_window_size"`
	// CollectionInterval determines how frequently system metrics are collected
	CollectionInterval time.Duration `json:"collection_interval"`

	// API configuration - controls the REST API server for sidecars
	// APIPort is the port number for the REST API server
	APIPort int `json:"api_port"`
	// APIKey is the authentication token required for sidecar requests
	APIKey string `json:"api_key"`
	// SwaggerEnable controls whether Swagger documentation is available
	SwaggerEnable bool `json:"swagger_enable"`

	// Prometheus configuration - controls metrics export
	// MetricsPort is the port number for the Prometheus metrics server
	MetricsPort int `json:"metrics_port"`
	// MetricsPath is the HTTP path for metrics endpoint
	MetricsPath string `json:"metrics_path"`

	// Kubernetes configuration - controls cluster integration
	// NodeName identifies which node this daemon is running on
	NodeName string `json:"node_name"`
	// PodNamespace is the namespace where this daemon pod is running
	PodNamespace string `json:"pod_namespace"`
	// KubeConfig is the path to kubeconfig file (optional, uses in-cluster config by default)
	KubeConfig string `json:"kube_config"`

	// Output configuration - controls incident report formatting
	// OutputFormatters is a list of formatters to use for incident reports
	OutputFormatters []string `json:"output_formatters"`
	// OutputPath is the directory or destination for incident reports
	OutputPath string `json:"output_path"`

	// Emitter configuration - controls where formatted logs are emitted
	// Emitters is a list of emitter configurations for sending formatted logs to various destinations
	Emitters []emitter.EmitterConfig `json:"emitters"`

	// Logging configuration - controls daemon logging behavior
	// LogLevel controls the verbosity of logging (debug, info, warn, error)
	LogLevel string `json:"log_level"`
	// LogJSON controls whether logs are formatted as JSON
	LogJSON bool `json:"log_json"`
}

// DefaultConfig returns a configuration with sensible defaults for production use.
// These defaults prioritize performance and security while providing comprehensive monitoring.
func DefaultConfig() *Config {
	return &Config{
		BufferWindowSize:   60 * time.Second,
		CollectionInterval: 1 * time.Second,
		APIPort:            8080,
		SwaggerEnable:      false,
		MetricsPort:        9090,
		MetricsPath:        "/metrics",
		OutputFormatters:   []string{"default"},
		OutputPath:         "/var/log/blackbox",
		Emitters: []emitter.EmitterConfig{
			{
				Type: "file",
				Config: map[string]interface{}{
					"path":        "/var/log/blackbox/incidents.log",
					"create_dirs": true,
					"append":      true,
				},
			},
		},
		LogLevel: "info",
		LogJSON:  true,
	}
}

// LoadFromEnv loads configuration from environment variables with fallback to defaults.
// LoadFromEnv creates a new Config instance by reading from environment variables.
// This function reads all supported environment variables and validates their values.
// Returns an error if any configuration value is invalid.
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	// Buffer configuration
	if val := os.Getenv("BLACKBOX_BUFFER_WINDOW_SIZE"); val != "" {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_BUFFER_WINDOW_SIZE: %w", err)
		}
		cfg.BufferWindowSize = duration
	}

	if val := os.Getenv("BLACKBOX_COLLECTION_INTERVAL"); val != "" {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_COLLECTION_INTERVAL: %w", err)
		}
		cfg.CollectionInterval = duration
	}

	// API configuration
	if val := os.Getenv("BLACKBOX_API_PORT"); val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_API_PORT: %w", err)
		}
		cfg.APIPort = port
	}

	if val := os.Getenv("BLACKBOX_API_KEY"); val != "" {
		cfg.APIKey = val
	}

	if val := os.Getenv("BLACKBOX_SWAGGER_ENABLE"); val != "" {
		enable, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_SWAGGER_ENABLE: %w", err)
		}
		cfg.SwaggerEnable = enable
	}

	// Prometheus configuration
	if val := os.Getenv("BLACKBOX_METRICS_PORT"); val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_METRICS_PORT: %w", err)
		}
		cfg.MetricsPort = port
	}

	if val := os.Getenv("BLACKBOX_METRICS_PATH"); val != "" {
		cfg.MetricsPath = val
	}

	// Kubernetes configuration
	if val := os.Getenv("NODE_NAME"); val != "" {
		cfg.NodeName = val
	}

	if val := os.Getenv("POD_NAMESPACE"); val != "" {
		cfg.PodNamespace = val
	}

	if val := os.Getenv("KUBECONFIG"); val != "" {
		cfg.KubeConfig = val
	}

	// Output configuration
	if val := os.Getenv("BLACKBOX_OUTPUT_FORMATTERS"); val != "" {
		cfg.OutputFormatters = strings.Split(val, ",")
		for i, formatter := range cfg.OutputFormatters {
			cfg.OutputFormatters[i] = strings.TrimSpace(formatter)
		}
	}

	if val := os.Getenv("BLACKBOX_OUTPUT_PATH"); val != "" {
		cfg.OutputPath = val
	}

	// Emitter configuration
	if val := os.Getenv("BLACKBOX_EMITTERS"); val != "" {
		var emitterConfigs []emitter.EmitterConfig
		if err := json.Unmarshal([]byte(val), &emitterConfigs); err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_EMITTERS JSON: %w", err)
		}
		cfg.Emitters = emitterConfigs
	}

	// Logging configuration
	if val := os.Getenv("BLACKBOX_LOG_LEVEL"); val != "" {
		cfg.LogLevel = val
	}

	if val := os.Getenv("BLACKBOX_LOG_JSON"); val != "" {
		json, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid BLACKBOX_LOG_JSON: %w", err)
		}
		cfg.LogJSON = json
	}

	return cfg, nil
}

// Validate checks if the configuration is valid and returns an error if not.
// This performs comprehensive validation of all configuration parameters to ensure
// the daemon can start successfully with the provided configuration.
func (c *Config) Validate() error {
	if c.BufferWindowSize <= 0 {
		return fmt.Errorf("buffer window size must be positive")
	}

	if c.CollectionInterval <= 0 {
		return fmt.Errorf("collection interval must be positive")
	}

	if c.APIPort <= 0 || c.APIPort > 65535 {
		return fmt.Errorf("API port must be between 1 and 65535")
	}

	if c.MetricsPort <= 0 || c.MetricsPort > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535")
	}

	if c.APIKey == "" {
		return fmt.Errorf("API key is required for sidecar authentication")
	}

	if len(c.OutputFormatters) == 0 {
		return fmt.Errorf("at least one output formatter must be specified")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}

	// Validate emitter configurations
	if len(c.Emitters) == 0 {
		return fmt.Errorf("at least one emitter must be configured")
	}
	
	for i, emitterConfig := range c.Emitters {
		if emitterConfig.Type == "" {
			return fmt.Errorf("emitter %d: type is required", i)
		}
		// Validate that we can create the emitter (tests registry availability)
		if _, err := emitter.CreateEmitter(emitterConfig); err != nil {
			return fmt.Errorf("emitter %d (%s): %w", i, emitterConfig.Type, err)
		}
	}

	return nil
}
