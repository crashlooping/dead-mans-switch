package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	if err := os.WriteFile("test.yaml", []byte(`listen_addr: ":1234"
timeout_seconds: 5
notification_channels:
  - type: dummy
    to: "a@b.com"
notification_messages:
  timeout: "Timeout for {{name}}!"
  recovery: "Recovery for {{name}}!"
`), 0644); err != nil {
		t.Fatalf("failed to write test.yaml: %v", err)
	}
	cfg, err := LoadConfig("test.yaml")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if cfg.ListenAddr != ":1234" {
		t.Errorf("listen_addr not loaded")
	}
	if cfg.TimeoutSeconds != 5 {
		t.Errorf("timeout_seconds not loaded")
	}
	if len(cfg.NotificationChannels) != 1 || cfg.NotificationChannels[0].Type != "dummy" {
		t.Errorf("notification_channels not loaded")
	}
	if cfg.NotificationChannels[0].Properties["to"] != "a@b.com" {
		t.Errorf("notification channel property not loaded")
	}
	if cfg.NotificationMessages.Timeout != "Timeout for {{name}}!" {
		t.Errorf("notification_messages.timeout not loaded")
	}
	if cfg.NotificationMessages.Recovery != "Recovery for {{name}}!" {
		t.Errorf("notification_messages.recovery not loaded")
	}
	if err := os.Remove("test.yaml"); err != nil {
		t.Errorf("os.Remove error: %v", err)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	if err := os.WriteFile("test_defaults.yaml", []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write test_defaults.yaml: %v", err)
	}
	defer os.Remove("test_defaults.yaml")

	cfg, err := LoadConfig("test_defaults.yaml")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("expected default listen_addr ':8080', got '%s'", cfg.ListenAddr)
	}
	if cfg.TimeoutSeconds != 600 {
		t.Errorf("expected default timeout_seconds 600, got %d", cfg.TimeoutSeconds)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	// LoadConfig silently handles missing files and returns defaults
	cfg, err := LoadConfig("nonexistent_config_12345.yaml")
	if err != nil {
		t.Fatalf("expected no error for nonexistent config file, got: %v", err)
	}
	if cfg == nil {
		t.Error("expected config with defaults, got nil")
	}
	// Should have defaults
	if cfg.ListenAddr != ":8080" {
		t.Errorf("expected default listen_addr, got %s", cfg.ListenAddr)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	if err := os.WriteFile("test_invalid.yaml", []byte(`invalid: yaml: content: [[[`), 0644); err != nil {
		t.Fatalf("failed to write test_invalid.yaml: %v", err)
	}
	defer os.Remove("test_invalid.yaml")

	_, err := LoadConfig("test_invalid.yaml")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestTimeout(t *testing.T) {
	cfg := &Config{TimeoutSeconds: 120}
	expected := time.Duration(120) * time.Second
	if cfg.Timeout() != expected {
		t.Errorf("expected %v, got %v", expected, cfg.Timeout())
	}
}

func TestTimeoutZero(t *testing.T) {
	cfg := &Config{TimeoutSeconds: 0}
	expected := time.Duration(0) * time.Second
	if cfg.Timeout() != expected {
		t.Errorf("expected %v, got %v", expected, cfg.Timeout())
	}
}
