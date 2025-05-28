package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"syslog_webhook_to_kafka/common"

	"gopkg.in/yaml.v3"
)

type KafkaConfig struct {
	ID      string   `yaml:"id"`
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	Key     string   `yaml:"key,omitempty"`
}

type SyslogServerConfig struct {
	Listen   string `yaml:"listen"`
	Format   string `yaml:"format"`
	Protocol string `yaml:"protocol"`
	KafkaID  string `yaml:"kafka_id"`
	//Grok     string `yaml:"grok"`
	//
	//NamedCapturesOnly   bool `yaml:"named_captures_only,omitempty"`
	//SkipDefaultPatterns bool `yaml:"skip_default_patterns,omitempty"`
	//RemoveEmptyValues   bool `yaml:"remove_empty_values,omitempty"`
}

type WebhookTLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type WebhookConfig struct {
	Listen  string           `yaml:"listen"`
	Path    string           `yaml:"path"`
	TLS     WebhookTLSConfig `yaml:"tls,omitempty"`
	KafkaID string           `yaml:"kafka_id"`
}

type Config struct {
	Kafka   []KafkaConfig        `yaml:"kafka"`
	Syslog  []SyslogServerConfig `yaml:"syslog"`
	Webhook []WebhookConfig      `yaml:"webhook"`
}

func validateConfig(config *Config) error {
	// Validate Kafka configurations
	if len(config.Kafka) == 0 {
		return fmt.Errorf("kafka configuration is required")
	}
	for i, k := range config.Kafka {
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
	if len(config.Syslog) == 0 {
		return fmt.Errorf("syslog configuration is required")
	}
	for i, s := range config.Syslog {
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
		//if s.Grok == "" {
		//	return fmt.Errorf("syslog[%d]: grok is required", i)
		//}
		// Validate that kafka_id exists in kafka configs
		found := false
		for _, k := range config.Kafka {
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
	if len(config.Webhook) == 0 {
		return fmt.Errorf("webhook configuration is required")
	}
	for i, w := range config.Webhook {
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
		for _, k := range config.Kafka {
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

func main() {
	// Read config file
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		fmt.Printf("Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Create message channels for each Kafka instance
	msgChans := make(map[string]chan []byte)
	for _, kc := range config.Kafka {
		msgChans[kc.ID] = make(chan []byte, 100)
	}

	// Initialize Kafka producers
	kafkaProducers := make(map[string]*common.KafkaProducer)
	for _, kc := range config.Kafka {
		var keyField = []string{}
		if kc.Key != "" {
			keyField = common.StringToList(kc.Key)
		}

		producer, err := common.NewKafkaProducer(kc.Brokers, kc.Topic, keyField)
		if err != nil {
			fmt.Printf("Error creating Kafka producer: %v\n", err)
			os.Exit(1)
		}
		kafkaProducers[kc.ID] = producer
		fmt.Printf("[INFO] Kafka producer initialized: id=%s, topic=%s\n", kc.ID, kc.Topic)

		// Start message consumer for this Kafka instance
		go func(kafkaID string, msgChan chan []byte) {
			fmt.Printf("[INFO] Starting message consumer for Kafka: %s\n", kafkaID)
			for msg := range msgChan {
				if err := kafkaProducers[kafkaID].SendMessage(msg); err != nil {
					fmt.Printf("Error sending message to Kafka %s: %v\n", kafkaID, err)
				}
			}
		}(kc.ID, msgChans[kc.ID])
	}

	// Initialize syslog servers
	var syslogServers []*common.SyslogConfig
	for _, sc := range config.Syslog {
		server, err := common.NewSyslog(sc.Listen, sc.Protocol, sc.Format, msgChans[sc.KafkaID])
		if err != nil {
			fmt.Printf("Error creating syslog server: %v\n", err)
			os.Exit(1)
		}
		syslogServers = append(syslogServers, server)
		fmt.Printf("[INFO] Starting syslog server: listen=%s, protocol=%s\n", sc.Listen, sc.Protocol)
		go server.Run()
	}

	// Initialize webhook servers
	var webhookServers []*common.WebhookServer
	for _, wc := range config.Webhook {
		var tlsConfig *common.WebhookTLSConfig
		if strings.HasPrefix(wc.Listen, "https://") {
			tlsConfig = &common.WebhookTLSConfig{
				Enabled:  wc.TLS.Enabled,
				CertFile: wc.TLS.CertFile,
				KeyFile:  wc.TLS.KeyFile,
			}
		}
		server, err := common.NewWebhook(wc.Listen, wc.Path, msgChans[wc.KafkaID], tlsConfig)
		if err != nil {
			fmt.Printf("Error creating webhook server: %v\n", err)
			os.Exit(1)
		}
		webhookServers = append(webhookServers, server)
		fmt.Printf("[INFO] Starting webhook server: listen=%s, path=%s\n", wc.Listen, wc.Path)
	}

	// Start webhook servers
	for _, server := range webhookServers {
		go func(s *common.WebhookServer) {
			if err := s.Run(); err != nil {
				fmt.Printf("Webhook server error: %v\n", err)
				os.Exit(1)
			}
		}(server)
	}

	fmt.Println("[INFO] All servers started successfully")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n[INFO] Received shutdown signal, stopping servers...")

	// Stop all webhook servers
	for _, server := range webhookServers {
		if err := server.Stop(); err != nil {
			fmt.Printf("Error stopping webhook server: %v\n", err)
		}
	}

	// Stop all syslog servers
	for _, server := range syslogServers {
		server.Stop()
	}

	// Close all Kafka producers
	for _, producer := range kafkaProducers {
		if err := producer.Close(); err != nil {
			fmt.Printf("Error closing Kafka producer: %v\n", err)
		}
	}

	fmt.Println("[INFO] All servers stopped successfully")
}
