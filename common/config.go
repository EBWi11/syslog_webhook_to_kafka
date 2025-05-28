package common

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Kafka   []KafkaConfig        `yaml:"kafka"`
	Syslog  []SyslogServerConfig `yaml:"syslog"`
	Webhook []WebhookConfig      `yaml:"webhook"`
}

// SyslogServerConfig represents the syslog server configuration
type SyslogServerConfig struct {
	Listen   string `yaml:"listen"`
	Format   string `yaml:"format"`
	Protocol string `yaml:"protocol"`
	KafkaID  string `yaml:"kafka_id"`
}

// WebhookServerConfig represents the webhook server configuration
type WebhookConfig struct {
	Listen  string           `yaml:"listen"`
	Path    string           `yaml:"path"`
	TLS     WebhookTLSConfig `yaml:"tls,omitempty"`
	KafkaID string           `yaml:"kafka_id"`
}

// TLSConfig represents the TLS configuration
type WebhookTLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// KafkaConfig represents the Kafka configuration
type KafkaConfig struct {
	ID      string   `yaml:"id"`
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	Key     *struct {
		Field string `yaml:"field"` // JSON field to use as key
		Type  string `yaml:"type"`  // Type of key: string, number, timestamp
	} `yaml:"key,omitempty"`
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Kafka configurations
	if len(c.Kafka) == 0 {
		return fmt.Errorf("kafka configuration is required")
	}
	for i, k := range c.Kafka {
		if k.ID == "" {
			return fmt.Errorf("kafka[%d]: id is required", i)
		}
		if len(k.Brokers) == 0 {
			return fmt.Errorf("kafka[%d]: at least one broker is required", i)
		}
		if k.Topic == "" {
			return fmt.Errorf("kafka[%d]: topic is required", i)
		}
	}

	// Validate Syslog configurations
	if len(c.Syslog) == 0 {
		return fmt.Errorf("syslog configuration is required")
	}
	for i, s := range c.Syslog {
		if s.Listen == "" {
			return fmt.Errorf("syslog[%d]: listen address is required", i)
		}
		if s.Format == "" {
			return fmt.Errorf("syslog[%d]: format is required", i)
		}
		if s.Protocol == "" {
			return fmt.Errorf("syslog[%d]: protocol is required", i)
		}
		if s.KafkaID == "" {
			return fmt.Errorf("syslog[%d]: kafka_id is required", i)
		}
		// Validate that kafka_id exists in kafka configs
		found := false
		for _, k := range c.Kafka {
			if k.ID == s.KafkaID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("syslog[%d]: kafka_id '%s' not found in kafka configurations", i, s.KafkaID)
		}
	}

	// Validate Webhook configurations
	if len(c.Webhook) == 0 {
		return fmt.Errorf("webhook configuration is required")
	}
	for i, w := range c.Webhook {
		if w.Listen == "" {
			return fmt.Errorf("webhook[%d]: listen address is required", i)
		}
		if w.Path == "" {
			return fmt.Errorf("webhook[%d]: path is required", i)
		}
		if w.KafkaID == "" {
			return fmt.Errorf("webhook[%d]: kafka_id is required", i)
		}
		// Validate that kafka_id exists in kafka configs
		found := false
		for _, k := range c.Kafka {
			if k.ID == w.KafkaID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("webhook[%d]: kafka_id '%s' not found in kafka configurations", i, w.KafkaID)
		}
		// Validate TLS configuration for HTTPS
		if strings.HasPrefix(w.Listen, "https://") {
			if !w.TLS.Enabled {
				return fmt.Errorf("webhook[%d]: TLS must be enabled for HTTPS", i)
			}
			if w.TLS.CertFile == "" || w.TLS.KeyFile == "" {
				return fmt.Errorf("webhook[%d]: both cert_file and key_file are required for HTTPS", i)
			}
		}
	}

	return nil
}
