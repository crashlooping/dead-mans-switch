package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testConfigPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func TestLoadConfig(t *testing.T) {
	path := testConfigPath(t, "test.yaml")
	if err := os.WriteFile(path, []byte(`listen_addr: ":1234"
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
	cfg, err := LoadConfig(path)
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
}

func TestLoadConfigDefaults(t *testing.T) {
	path := testConfigPath(t, "test_defaults.yaml")
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write test_defaults.yaml: %v", err)
	}

	cfg, err := LoadConfig(path)
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
	path := testConfigPath(t, "test_invalid.yaml")
	if err := os.WriteFile(path, []byte(`invalid: yaml: content: [[[`), 0644); err != nil {
		t.Fatalf("failed to write test_invalid.yaml: %v", err)
	}

	_, err := LoadConfig(path)
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

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"abc123XYZ789", "abc***789"},
		{"short", "***"},
		{"123456789", "123***789"},
		{"123456", "***"},
		{"", "***"},
	}
	for _, tt := range tests {
		got := MaskValue(tt.input)
		if got != tt.want {
			t.Errorf("MaskValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMaskChannelSecrets(t *testing.T) {
	channels := []NotificationChannel{
		{
			Type: "smtp",
			Properties: map[string]string{
				"smtp_server": "smtp.example.com",
				"smtp_user":   "user@example.com",
				"smtp_pass":   "supersecret",
				"to":          "recipient@example.com",
			},
		},
		{
			Type: "telegram",
			Properties: map[string]string{
				"bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				"chat_id":   "-123456789",
			},
		},
	}
	masked := MaskChannelSecrets(channels)
	// Original must not be mutated
	if channels[0].Properties["smtp_pass"] != "supersecret" {
		t.Error("MaskChannelSecrets mutated original channels")
	}
	// smtp_pass must be masked (partially, since >6 chars)
	if masked[0].Properties["smtp_pass"] == "supersecret" {
		t.Errorf("smtp_pass not masked")
	}
	// smtp_server must remain unmasked
	if masked[0].Properties["smtp_server"] != "smtp.example.com" {
		t.Errorf("smtp_server was masked: got %q", masked[0].Properties["smtp_server"])
	}
	// bot_token must be partially masked
	if masked[1].Properties["bot_token"] == "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" {
		t.Error("bot_token not masked")
	}
	// chat_id must remain unmasked
	if masked[1].Properties["chat_id"] != "-123456789" {
		t.Errorf("chat_id was masked: got %q", masked[1].Properties["chat_id"])
	}
}

func TestMaskChannelSecretsEmpty(t *testing.T) {
	masked := MaskChannelSecrets(nil)
	if masked != nil {
		t.Errorf("expected nil, got %v", masked)
	}
	masked = MaskChannelSecrets([]NotificationChannel{})
	if masked != nil {
		t.Errorf("expected nil for empty input, got %v", masked)
	}
}

func TestTimeoutZero(t *testing.T) {
	cfg := &Config{TimeoutSeconds: 0}
	expected := time.Duration(0) * time.Second
	if cfg.Timeout() != expected {
		t.Errorf("expected %v, got %v", expected, cfg.Timeout())
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	path := testConfigPath(t, "test_env.yaml")
	if err := os.WriteFile(path, []byte(`listen_addr: ":9999"
timeout_seconds: 300
invert: false
`), 0644); err != nil {
		t.Fatalf("failed to write test_env.yaml: %v", err)
	}

	// Set env vars to override file values
	t.Setenv("LISTEN_ADDR", ":1111")
	t.Setenv("TIMEOUT_SECONDS", "900")
	t.Setenv("INVERT", "true")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if cfg.ListenAddr != ":1111" {
		t.Errorf("LISTEN_ADDR not overridden: got %q", cfg.ListenAddr)
	}
	if cfg.TimeoutSeconds != 900 {
		t.Errorf("TIMEOUT_SECONDS not overridden: got %d", cfg.TimeoutSeconds)
	}
	if !cfg.Invert {
		t.Error("INVERT not overridden: expected true")
	}
}

func TestLoadConfigEnvOnly(t *testing.T) {
	// Test loading config entirely from env vars with an empty/minimal YAML file
	path := testConfigPath(t, "test_env_only.yaml")
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write test_env_only.yaml: %v", err)
	}

	t.Setenv("LISTEN_ADDR", ":7777")
	t.Setenv("TIMEOUT_SECONDS", "120")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if cfg.ListenAddr != ":7777" {
		t.Errorf("expected LISTEN_ADDR ':7777', got %q", cfg.ListenAddr)
	}
	if cfg.TimeoutSeconds != 120 {
		t.Errorf("expected TIMEOUT_SECONDS 120, got %d", cfg.TimeoutSeconds)
	}
	// Default values should still apply for unset fields
	if cfg.Invert {
		t.Error("expected INVERT to remain false")
	}
}
