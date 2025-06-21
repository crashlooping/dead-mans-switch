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
