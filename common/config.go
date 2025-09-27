package common

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	App struct {
		Name        string `yaml:"name"`
		Environment string `yaml:"environment"`
		Port        int    `yaml:"port"`
		LogLevel    string `yaml:"log_level"`
	} `yaml:"app"`

	Services struct {
		Catalog struct {
			Port int `yaml:"port"`
		} `yaml:"catalog"`
		Cart struct {
			Port int `yaml:"port"`
		} `yaml:"cart"`
		Payment struct {
			Port       int      `yaml:"port"`
			Processors []string `yaml:"processors"`
		} `yaml:"payment"`
		Shipping struct {
			Port     int      `yaml:"port"`
			Carriers []string `yaml:"carriers"`
		} `yaml:"shipping"`
		Image struct {
			Port int `yaml:"port"`
		} `yaml:"image"`
		Recommendations struct {
			Port int `yaml:"port"`
		} `yaml:"recommendations"`
	} `yaml:"services"`

	FaultInjection struct {
		PaymentFailureRate  float64 `yaml:"payment_failure_rate"`
		ShippingFailureRate float64 `yaml:"shipping_failure_rate"`
	} `yaml:"fault_injection"`

	Telemetry struct {
		Enabled     bool   `yaml:"enabled"`
		ServiceName string `yaml:"service_name"`
	} `yaml:"telemetry"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}

// GetServiceURL returns the URL for a given service
func (c *Config) GetServiceURL(service string) string {
	switch service {
	case "catalog":
		return fmt.Sprintf("http://localhost:%d", c.Services.Catalog.Port)
	case "cart":
		return fmt.Sprintf("http://localhost:%d", c.Services.Cart.Port)
	case "payment":
		return fmt.Sprintf("http://localhost:%d", c.Services.Payment.Port)
	case "shipping":
		return fmt.Sprintf("http://localhost:%d", c.Services.Shipping.Port)
	case "image":
		return fmt.Sprintf("http://localhost:%d", c.Services.Image.Port)
	case "recommendations":
		return fmt.Sprintf("http://localhost:%d", c.Services.Recommendations.Port)
	default:
		return ""
	}
}