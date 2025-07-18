package config

import (
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

type NotificationChannel struct {
	Type       string            `yaml:"type" envconfig:"TYPE"`
	Properties map[string]string `yaml:",inline"`
}

type NotificationMessages struct {
	Timeout  string `yaml:"timeout" envconfig:"NOTIFY_TIMEOUT_MSG"`
	Recovery string `yaml:"recovery" envconfig:"NOTIFY_RECOVERY_MSG"`
}

type Config struct {
	ListenAddr           string                `yaml:"listen_addr" envconfig:"LISTEN_ADDR"`
	TimeoutSeconds       int                   `yaml:"timeout_seconds" envconfig:"TIMEOUT_SECONDS"`
	NotificationChannels []NotificationChannel `yaml:"notification_channels"`
	NotificationMessages NotificationMessages  `yaml:"notification_messages"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:     ":8080",
		TimeoutSeconds: 600,
	}
	file, err := os.Open(path)
	if err == nil {
		defer func() {
			_ = file.Close()
		}()
		if err := yaml.NewDecoder(file).Decode(cfg); err != nil {
			// Log or handle YAML decode error
			return nil, err
		}
	}
	// ENV overrides
	if err := envconfig.Process("", cfg); err != nil {
		// Log or handle envconfig error
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}
