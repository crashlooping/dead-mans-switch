package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	if err := os.WriteFile("test.yaml", []byte(`listen_addr: ":1234"
timeout_seconds: 5
notification_channels:
  - type: dummy
    to: "a@b.com"
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
	if err := os.Remove("test.yaml"); err != nil {
		t.Errorf("os.Remove error: %v", err)
	}
}
