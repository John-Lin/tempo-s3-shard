package config

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	ListenAddr      string   `json:"listen_addr"`
	Endpoint        string   `json:"endpoint"`
	AccessKeyID     string   `json:"access_key_id"`
	SecretAccessKey string   `json:"secret_access_key"`
	UseSSL          bool     `json:"use_ssl"`
	Region          string   `json:"region"`
	Buckets         []string `json:"buckets"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ParsedEndpoint returns the host and SSL setting from the endpoint
func (c *Config) ParsedEndpoint() (host string, useSSL bool, err error) {
	endpoint := c.Endpoint
	
	// If endpoint doesn't have a scheme, use the UseSSL field
	if !strings.Contains(endpoint, "://") {
		return endpoint, c.UseSSL, nil
	}
	
	// Parse the URL to extract scheme and host
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, err
	}
	
	host = u.Host
	useSSL = u.Scheme == "https"
	
	return host, useSSL, nil
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddr:      ":8080",
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UseSSL:          false,
		Region:          "us-east-1",
		Buckets:         []string{"bucket1", "bucket2", "bucket3"},
	}
}