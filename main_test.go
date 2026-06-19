package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/config"
	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/db"
)

func TestHeartbeatEndpoint(t *testing.T) {
	dbPath := t.TempDir() + "/test-heartbeats.db"
	dbInstance, _ = db.Open(dbPath)
	defer func() {
		if err := dbInstance.Close(); err != nil {
			t.Errorf("dbInstance.Close() error: %v", err)
		}
	}()

	h := http.NewServeMux()
	h.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		type req struct {
			Name string `json:"name"`
		}
		var body req
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := dbInstance.UpdateHeartbeat(body.Name, time.Now(), false); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/heartbeat", "application/json", strings.NewReader(`{"name":"testclient"}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHeartbeatEndpointBadRequest(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		type req struct {
			Name string `json:"name"`
		}
		var body req
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Missing or invalid 'name' in body"))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/heartbeat", "application/json", strings.NewReader(`{"name":""}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHeartbeatEndpointMethodNotAllowed(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/heartbeat")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestHealthEndpoint(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/up", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	ts := httptest.NewServer(h)
	defer func() {
		ts.Close()
	}()

	resp, err := http.Get(ts.URL + "/up")
	if err != nil {
		t.Fatalf("GET /up failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	buf := make([]byte, 16)
	n, _ := resp.Body.Read(buf)
	if string(buf[:n]) != "ok" {
		t.Errorf("expected body 'ok', got '%s'", string(buf[:n]))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"seconds only", 45 * time.Second, "45s"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "2m30s"},
		{"hours minutes and seconds", 1*time.Hour + 15*time.Minute + 45*time.Second, "1h15m45s"},
		{"zero duration", 0 * time.Second, "0s"},
		{"one hour", 1 * time.Hour, "1h0m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestHeartbeatEndpointJSON(t *testing.T) {
	dbPath := t.TempDir() + "/test-heartbeats-json.db"
	dbInstance, _ = db.Open(dbPath)
	defer func() {
		dbInstance.Close()
	}()

	h := http.NewServeMux()
	h.HandleFunc("/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		heartbeats, err := dbInstance.GetAllHeartbeats()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("DB error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(heartbeats); err != nil {
			t.Logf("Encode error: %v", err)
		}
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Add a heartbeat first
	dbInstance.UpdateHeartbeat("test-client", time.Now(), false)

	resp, err := http.Get(ts.URL + "/heartbeats")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var heartbeats map[string]db.ClientHeartbeat
	if err := json.NewDecoder(resp.Body).Decode(&heartbeats); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	resp.Body.Close()

	if _, ok := heartbeats["test-client"]; !ok {
		t.Error("expected heartbeat for test-client not found")
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	cfg := &config.Config{
		SecurityHeaders: config.SecurityHeaders{
			XContentTypeOptions: "nosniff",
			XFrameOptions:       "DENY",
			ReferrerPolicy:      "strict-origin-when-cross-origin",
		},
	}
	ts := httptest.NewServer(securityHeaders(cfg, h))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want 'nosniff'", got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want 'DENY'", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want 'strict-origin-when-cross-origin'", got)
	}
}

func TestSecurityHeadersSameOrigin(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	cfg := &config.Config{
		SecurityHeaders: config.SecurityHeaders{
			XContentTypeOptions: "nosniff",
			XFrameOptions:       "SAMEORIGIN",
			ReferrerPolicy:      "no-referrer",
		},
	}
	ts := httptest.NewServer(securityHeaders(cfg, h))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want 'SAMEORIGIN'", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want 'no-referrer'", got)
	}
}

func TestSecurityHeadersDisabled(t *testing.T) {
	h := http.NewServeMux()
	h.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	cfg := &config.Config{
		SecurityHeaders: config.SecurityHeaders{
			XContentTypeOptions: "off",
			XFrameOptions:       "off",
			ReferrerPolicy:      "off",
		},
	}
	ts := httptest.NewServer(securityHeaders(cfg, h))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Content-Type-Options"); got != "" {
		t.Errorf("X-Content-Type-Options = %q, want empty (disabled)", got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options = %q, want empty (disabled)", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "" {
		t.Errorf("Referrer-Policy = %q, want empty (disabled)", got)
	}
}

func TestSetupNotifiers(t *testing.T) {
	cfg := &config.Config{
		NotificationChannels: []config.NotificationChannel{
			{Type: "dummy", Properties: map[string]string{"to": "test@example.com"}},
		},
	}
	notifiers := setupNotifiers(cfg)
	if len(notifiers) != 1 {
		t.Fatalf("expected 1 notifier, got %d", len(notifiers))
	}
}

func TestSetupNotifiersUnknown(t *testing.T) {
	cfg := &config.Config{
		NotificationChannels: []config.NotificationChannel{
			{Type: "nonexistent", Properties: map[string]string{}},
		},
	}
	notifiers := setupNotifiers(cfg)
	if len(notifiers) != 0 {
		t.Errorf("expected 0 notifiers for unknown type, got %d", len(notifiers))
	}
}

func TestGenerateDeviceTableNormal(t *testing.T) {
	cfg := &config.Config{Invert: false}
	now := time.Now().Truncate(time.Second)
	heartbeats := map[string]db.ClientHeartbeat{
		"alpha": {Name: "alpha", Timestamp: now, Missing: false},
		"beta":  {Name: "beta", Timestamp: now.Add(-5 * time.Minute), Missing: true},
	}
	html := generateDeviceTable(cfg, heartbeats)

	// Table structure
	if !strings.Contains(html, "<table>") {
		t.Error("expected <table> tag")
	}
	if !strings.Contains(html, "Missing") {
		t.Error("expected 'Missing' column header in normal mode")
	}
	if strings.Contains(html, "Available") {
		t.Error("should not contain 'Available' header in normal mode")
	}
	// Devices should be sorted alphabetically
	alphaIdx := strings.Index(html, "alpha")
	betaIdx := strings.Index(html, "beta")
	if alphaIdx < 0 || betaIdx < 0 {
		t.Fatal("expected both device names in output")
	}
	if alphaIdx > betaIdx {
		t.Error("devices should be sorted alphabetically (alpha before beta)")
	}
	// Normal mode: missing=true -> "yes" with status-yes
	if !strings.Contains(html, "status-yes") {
		t.Error("expected status-yes class for missing device")
	}
}

func TestGenerateDeviceTableInvert(t *testing.T) {
	cfg := &config.Config{Invert: true}
	now := time.Now().Truncate(time.Second)
	heartbeats := map[string]db.ClientHeartbeat{
		"device1": {Name: "device1", Timestamp: now, Missing: false},
		"device2": {Name: "device2", Timestamp: now, Missing: true},
	}
	html := generateDeviceTable(cfg, heartbeats)

	if !strings.Contains(html, "Available") {
		t.Error("expected 'Available' column header in invert mode")
	}
	if strings.Contains(html, "<th>Missing</th>") {
		t.Error("should not contain 'Missing' header in invert mode")
	}
	// Invert mode: missing=true -> "no" (not available)
	if !strings.Contains(html, "<td class='status-yes'><span") {
		t.Error("expected status-yes for unavailable device in invert mode")
	}
}

func TestGenerateDeviceTableHTMLEscape(t *testing.T) {
	cfg := &config.Config{Invert: false}
	now := time.Now().Truncate(time.Second)
	heartbeats := map[string]db.ClientHeartbeat{
		"<script>alert(1)</script>": {Name: "<script>alert(1)</script>", Timestamp: now, Missing: false},
	}
	html := generateDeviceTable(cfg, heartbeats)

	if strings.Contains(html, "<script>") {
		t.Error("device name must be HTML-escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected escaped <script> in output")
	}
}

func TestGenerateDeviceTableEmpty(t *testing.T) {
	cfg := &config.Config{Invert: false}
	html := generateDeviceTable(cfg, map[string]db.ClientHeartbeat{})

	if !strings.Contains(html, "<table>") || !strings.Contains(html, "</table>") {
		t.Error("expected valid table structure even with no devices")
	}
	if !strings.Contains(html, "<tbody></tbody>") {
		t.Error("expected empty tbody for no devices")
	}
}
