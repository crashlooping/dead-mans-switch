package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	os.WriteFile("test.yaml", []byte(`listen_addr: ":1234"
timeout_seconds: 5
notification_channels:
  - type: dummy
    to: "a@b.com"
`), 0644)
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
	os.Remove("test.yaml")
}
