package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"BLACKBOX_API_KEY",
		"BLACKBOX_BUFFER_WINDOW_SIZE",
		"BLACKBOX_COLLECTION_INTERVAL",
		"BLACKBOX_API_PORT",
		"BLACKBOX_METRICS_PORT",
		"BLACKBOX_OUTPUT_FORMATTERS",
		"BLACKBOX_OUTPUT_PATH",
		"BLACKBOX_LOG_LEVEL",
		"BLACKBOX_SWAGGER_ENABLE",
		"BLACKBOX_LOG_JSON",
		"NODE_NAME",
		"POD_NAMESPACE",
	}
	
	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	
	// Restore environment after test
	defer func() {
		for env, value := range originalEnv {
			if value == "" {
				os.Unsetenv(env)
			} else {
				os.Setenv(env, value)
			}
		}
	}()

	t.Run("loads config from environment", func(t *testing.T) {
		// Set environment variables
		os.Setenv("BLACKBOX_BUFFER_WINDOW_SIZE", "2h")
		os.Setenv("BLACKBOX_API_PORT", "8080")
		os.Setenv("BLACKBOX_API_KEY", "test-key-123")
		defer func() {
			os.Unsetenv("BLACKBOX_BUFFER_WINDOW_SIZE")
			os.Unsetenv("BLACKBOX_API_PORT")
			os.Unsetenv("BLACKBOX_API_KEY")
		}()

		config, err := LoadFromEnv()

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if config.BufferWindowSize != 2*time.Hour {
			t.Errorf("Expected BufferWindowSize 2h, got %v", config.BufferWindowSize)
		}
		if config.APIPort != 8080 {
			t.Errorf("Expected APIPort 8080, got %v", config.APIPort)
		}
		if config.APIKey != "test-key-123" {
			t.Errorf("Expected APIKey 'test-key-123', got %v", config.APIKey)
		}
	})

	t.Run("loads without API key but fails validation", func(t *testing.T) {
		cfg, err := LoadFromEnv()
		
		if err != nil {
			t.Fatalf("LoadFromEnv should succeed, got %v", err)
		}
		
		// Validation should fail without API key
		err = cfg.Validate()
		if err == nil {
			t.Fatal("Expected validation to fail for missing API key")
		}
		if !strings.Contains(err.Error(), "API key is required") {
			t.Errorf("Expected API key error, got %v", err)
		}
	})

	t.Run("uses default values", func(t *testing.T) {
		os.Setenv("BLACKBOX_API_KEY", "test-key")
		defer os.Unsetenv("BLACKBOX_API_KEY")
		
		config, err := LoadFromEnv()
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check default values
		if config.BufferWindowSize != 60*time.Second {
			t.Errorf("Expected BufferWindowSize 60s, got %v", config.BufferWindowSize)
		}
		if config.CollectionInterval != 1*time.Second {
			t.Errorf("Expected CollectionInterval 1s, got %v", config.CollectionInterval)
		}
		if config.APIPort != 8080 {
			t.Errorf("Expected APIPort 8080, got %v", config.APIPort)
		}
		if config.MetricsPort != 9090 {
			t.Errorf("Expected MetricsPort 9090, got %v", config.MetricsPort)
		}
		if len(config.OutputFormatters) != 1 || config.OutputFormatters[0] != "default" {
			t.Errorf("Expected OutputFormatters ['default'], got %v", config.OutputFormatters)
		}
		if config.OutputPath != "/var/log/blackbox" {
			t.Errorf("Expected OutputPath '/var/log/blackbox', got %v", config.OutputPath)
		}
		if config.LogLevel != "info" {
			t.Errorf("Expected LogLevel 'info', got %v", config.LogLevel)
		}
		if config.SwaggerEnable != false {
			t.Errorf("Expected SwaggerEnable false, got %v", config.SwaggerEnable)
		}
		if config.LogJSON != true {
			t.Errorf("Expected LogJSON true, got %v", config.LogJSON)
		}
	})

	t.Run("parses custom values", func(t *testing.T) {
		os.Setenv("BLACKBOX_API_KEY", "custom-key")
		os.Setenv("BLACKBOX_BUFFER_WINDOW_SIZE", "5m")
		os.Setenv("BLACKBOX_COLLECTION_INTERVAL", "10s")
		os.Setenv("BLACKBOX_API_PORT", "9080")
		os.Setenv("BLACKBOX_METRICS_PORT", "9091")
		os.Setenv("BLACKBOX_OUTPUT_FORMATTERS", "json,csv")
		os.Setenv("BLACKBOX_OUTPUT_PATH", "/tmp/logs")
		os.Setenv("BLACKBOX_LOG_LEVEL", "debug")
		os.Setenv("BLACKBOX_SWAGGER_ENABLE", "true")
		os.Setenv("BLACKBOX_LOG_JSON", "false")
		os.Setenv("NODE_NAME", "test-node")
		os.Setenv("POD_NAMESPACE", "test-namespace")
		
		config, err := LoadFromEnv()
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if config.BufferWindowSize != 5*time.Minute {
			t.Errorf("Expected BufferWindowSize 5m, got %v", config.BufferWindowSize)
		}
		if config.CollectionInterval != 10*time.Second {
			t.Errorf("Expected CollectionInterval 10s, got %v", config.CollectionInterval)
		}
		if config.APIPort != 9080 {
			t.Errorf("Expected APIPort 9080, got %v", config.APIPort)
		}
		if config.MetricsPort != 9091 {
			t.Errorf("Expected MetricsPort 9091, got %v", config.MetricsPort)
		}
		if len(config.OutputFormatters) != 2 || config.OutputFormatters[0] != "json" || config.OutputFormatters[1] != "csv" {
			t.Errorf("Expected OutputFormatters ['json','csv'], got %v", config.OutputFormatters)
		}
		if config.OutputPath != "/tmp/logs" {
			t.Errorf("Expected OutputPath '/tmp/logs', got %v", config.OutputPath)
		}
		if config.LogLevel != "debug" {
			t.Errorf("Expected LogLevel 'debug', got %v", config.LogLevel)
		}
		if config.SwaggerEnable != true {
			t.Errorf("Expected SwaggerEnable true, got %v", config.SwaggerEnable)
		}
		if config.LogJSON != false {
			t.Errorf("Expected LogJSON false, got %v", config.LogJSON)
		}
		if config.NodeName != "test-node" {
			t.Errorf("Expected NodeName 'test-node', got %v", config.NodeName)
		}
		if config.PodNamespace != "test-namespace" {
			t.Errorf("Expected PodNamespace 'test-namespace', got %v", config.PodNamespace)
		}
	})

	t.Run("handles invalid duration formats", func(t *testing.T) {
		os.Setenv("BLACKBOX_API_KEY", "test-key")
		os.Setenv("BLACKBOX_BUFFER_WINDOW_SIZE", "invalid")
		
		_, err := LoadFromEnv()
		
		if err == nil {
			t.Fatal("Expected error for invalid duration")
		}
		if !strings.Contains(err.Error(), "invalid BLACKBOX_BUFFER_WINDOW_SIZE") {
			t.Errorf("Expected duration parsing error, got %v", err)
		}
	})

	t.Run("handles invalid port numbers", func(t *testing.T) {
		os.Setenv("BLACKBOX_API_KEY", "test-key")
		os.Setenv("BLACKBOX_API_PORT", "invalid")
		
		_, err := LoadFromEnv()
		
		if err == nil {
			t.Fatal("Expected error for invalid port")
		}
	})

	t.Run("handles invalid boolean values", func(t *testing.T) {
		os.Setenv("BLACKBOX_API_KEY", "test-key")
		os.Setenv("BLACKBOX_SWAGGER_ENABLE", "invalid")
		
		_, err := LoadFromEnv()
		
		if err == nil {
			t.Fatal("Expected error for invalid boolean")
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("validates valid config", func(t *testing.T) {
		config := &Config{
			APIKey:              "valid-api-key",
			BufferWindowSize:    60 * time.Second,
			CollectionInterval:  1 * time.Second,
			APIPort:            8080,
			MetricsPort:        9090,
			OutputFormatters:   []string{"json"},
			OutputPath:         "/var/log/blackbox",
			LogLevel:           "info",
			SwaggerEnable:      false,
			LogJSON:           true,
		}
		
		err := config.Validate()
		
		if err != nil {
			t.Errorf("Expected no error for valid config, got %v", err)
		}
	})

	t.Run("rejects empty API key", func(t *testing.T) {
		config := &Config{
			APIKey:              "",
			BufferWindowSize:    60 * time.Second,
			CollectionInterval:  1 * time.Second,
			APIPort:            8080,
			MetricsPort:        9090,
			OutputFormatters:   []string{"default"},
			LogLevel:           "info",
		}
		
		err := config.Validate()
		
		if err == nil {
			t.Fatal("Expected error for empty API key")
		}
		if !strings.Contains(err.Error(), "API key is required") {
			t.Errorf("Expected API key error, got %v", err)
		}
	})

	t.Run("rejects zero buffer window size", func(t *testing.T) {
		config := &Config{
			APIKey:              "valid-key",
			BufferWindowSize:    0,
			CollectionInterval:  1 * time.Second,
			APIPort:            8080,
			MetricsPort:        9090,
			OutputFormatters:   []string{"default"},
			LogLevel:           "info",
		}
		
		err := config.Validate()
		
		if err == nil {
			t.Fatal("Expected error for zero buffer window size")
		}
		if err.Error() != "buffer window size must be positive" {
			t.Errorf("Expected buffer window error, got %v", err)
		}
	})

	t.Run("rejects zero collection interval", func(t *testing.T) {
		config := &Config{
			APIKey:              "valid-key",
			BufferWindowSize:    60 * time.Second,
			CollectionInterval:  0,
			APIPort:            8080,
			MetricsPort:        9090,
			OutputFormatters:   []string{"default"},
			LogLevel:           "info",
		}
		
		err := config.Validate()
		
		if err == nil {
			t.Fatal("Expected error for zero collection interval")
		}
		if err.Error() != "collection interval must be positive" {
			t.Errorf("Expected collection interval error, got %v", err)
		}
	})

	t.Run("rejects invalid port numbers", func(t *testing.T) {
		testCases := []struct {
			name        string
			apiPort     int
			metricsPort int
			expectError string
		}{
			{"zero API port", 0, 9090, "API port must be between 1 and 65535"},
			{"negative API port", -1, 9090, "API port must be between 1 and 65535"},
			{"too high API port", 99999, 9090, "API port must be between 1 and 65535"},
			{"zero metrics port", 8080, 0, "metrics port must be between 1 and 65535"},
			{"negative metrics port", 8080, -1, "metrics port must be between 1 and 65535"},
			{"too high metrics port", 8080, 99999, "metrics port must be between 1 and 65535"},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &Config{
					APIKey:              "valid-key",
					BufferWindowSize:    60 * time.Second,
					CollectionInterval:  1 * time.Second,
					APIPort:            tc.apiPort,
					MetricsPort:        tc.metricsPort,
					OutputFormatters:   []string{"default"},
					LogLevel:           "info",
				}
				
				err := config.Validate()
				
				if err == nil {
					t.Fatalf("Expected error for %s", tc.name)
				}
				if err.Error() != tc.expectError {
					t.Errorf("Expected error '%s', got '%v'", tc.expectError, err)
				}
			})
		}
	})

	t.Run("rejects invalid log levels", func(t *testing.T) {
		config := &Config{
			APIKey:              "valid-key",
			BufferWindowSize:    60 * time.Second,
			CollectionInterval:  1 * time.Second,
			APIPort:            8080,
			MetricsPort:        9090,
			OutputFormatters:   []string{"default"},
			LogLevel:           "invalid",
		}
		
		err := config.Validate()
		
		if err == nil {
			t.Fatal("Expected error for invalid log level")
		}
		if !strings.Contains(err.Error(), "invalid log level") {
			t.Errorf("Expected log level error, got %v", err)
		}
	})

	t.Run("accepts valid log levels", func(t *testing.T) {
		validLevels := []string{"debug", "info", "warn", "error"}
		
		for _, level := range validLevels {
			t.Run(level, func(t *testing.T) {
				config := &Config{
					APIKey:              "valid-key",
					BufferWindowSize:    60 * time.Second,
					CollectionInterval:  1 * time.Second,
					APIPort:            8080,
					MetricsPort:        9090,
					OutputFormatters:   []string{"default"},
					LogLevel:           level,
				}
				
				err := config.Validate()
				
				if err != nil {
					t.Errorf("Expected no error for log level '%s', got %v", level, err)
				}
			})
		}
	})

	t.Run("rejects empty output formatters", func(t *testing.T) {
		config := &Config{
			APIKey:              "valid-key",
			BufferWindowSize:    60 * time.Second,
			CollectionInterval:  1 * time.Second,
			APIPort:            8080,
			MetricsPort:        9090,
			OutputFormatters:   []string{},
			LogLevel:           "info",
		}
		
		err := config.Validate()
		
		if err == nil {
			t.Fatal("Expected validation to fail for empty formatters")
		}
		if !strings.Contains(err.Error(), "at least one output formatter must be specified") {
			t.Errorf("Expected formatter error, got %v", err)
		}
	})
}

// TestDefaultConfig tests the DefaultConfig function to ensure proper defaults are set.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.BufferWindowSize != 60*time.Second {
		t.Errorf("Expected BufferWindowSize 60s, got %v", cfg.BufferWindowSize)
	}
	
	if cfg.CollectionInterval != 1*time.Second {
		t.Errorf("Expected CollectionInterval 1s, got %v", cfg.CollectionInterval)
	}
	
	if cfg.APIPort != 8080 {
		t.Errorf("Expected APIPort 8080, got %d", cfg.APIPort)
	}
	
	if cfg.MetricsPort != 9090 {
		t.Errorf("Expected MetricsPort 9090, got %d", cfg.MetricsPort)
	}
	
	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got %q", cfg.LogLevel)
	}
}