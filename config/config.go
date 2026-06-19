package config

import (
	"os"
	"strings"
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

type SecurityHeaders struct {
	XContentTypeOptions string `yaml:"x_content_type_options" envconfig:"X_CONTENT_TYPE_OPTIONS"`
	XFrameOptions       string `yaml:"x_frame_options" envconfig:"X_FRAME_OPTIONS"`
	ReferrerPolicy      string `yaml:"referrer_policy" envconfig:"REFERRER_POLICY"`
}

type Config struct {
	ListenAddr           string                `yaml:"listen_addr" envconfig:"LISTEN_ADDR"`
	TimeoutSeconds       int                   `yaml:"timeout_seconds" envconfig:"TIMEOUT_SECONDS"`
	Invert               bool                  `yaml:"invert" envconfig:"INVERT"`
	NotificationChannels []NotificationChannel `yaml:"notification_channels"`
	NotificationMessages NotificationMessages  `yaml:"notification_messages"`
	SecurityHeaders      SecurityHeaders       `yaml:"security_headers" envconfig:""`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:     ":8080",
		TimeoutSeconds: 600,
		SecurityHeaders: SecurityHeaders{
			XContentTypeOptions: "nosniff",
			XFrameOptions:       "DENY",
			ReferrerPolicy:      "strict-origin-when-cross-origin",
		},
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

// isSecretKey returns true if the property key should be masked.
func isSecretKey(k string) bool {
	switch {
	case k == "bot_token":
		return true
	case strings.Contains(k, "pass"), strings.Contains(k, "token"), strings.Contains(k, "secret"):
		return true
	}
	return false
}

// MaskValue returns a masked version of a secret value.
// If the value is longer than 6 characters, the first 3 and last 3 are kept.
func MaskValue(v string) string {
	if len(v) > 6 {
		return v[:3] + "***" + v[len(v)-3:]
	}
	return "***"
}

// MaskChannelSecrets returns a copy of channels with secret property values masked.
func MaskChannelSecrets(channels []NotificationChannel) []NotificationChannel {
	if len(channels) == 0 {
		return nil
	}
	masked := make([]NotificationChannel, len(channels))
	for i, ch := range channels {
		masked[i] = NotificationChannel{
			Type:       ch.Type,
			Properties: make(map[string]string),
		}
		for k, v := range ch.Properties {
			if isSecretKey(k) {
				masked[i].Properties[k] = MaskValue(v)
			} else {
				masked[i].Properties[k] = v
			}
		}
	}
	return masked
}
