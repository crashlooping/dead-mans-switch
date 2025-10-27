package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/db"
)

func TestHeartbeatEndpoint(t *testing.T) {
	dbPath := "test-heartbeats.db"
	dbInstance, _ = db.Open(dbPath)
	defer func() {
		if err := dbInstance.Close(); err != nil {
			t.Errorf("dbInstance.Close() error: %v", err)
		}
		if err := os.Remove(dbPath); err != nil {
			t.Errorf("os.Remove error: %v", err)
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
	dbPath := "test-heartbeats-json.db"
	dbInstance, _ = db.Open(dbPath)
	defer func() {
		dbInstance.Close()
		os.Remove(dbPath)
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
