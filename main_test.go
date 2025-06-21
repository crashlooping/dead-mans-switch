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

type notifierMock struct {
	calls []string
}

func (n *notifierMock) Notify(subject, message string) error {
	n.calls = append(n.calls, subject+":"+message)
	return nil
}

func TestHeartbeatEndpoint(t *testing.T) {
	dbPath := "test-heartbeats.db"
	dbInstance, _ = db.Open(dbPath)
	defer dbInstance.Close()
	defer os.Remove(dbPath)

	h := http.NewServeMux()
	h.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		type req struct {
			Name string `json:"name"`
		}
		var body req
		_ = json.NewDecoder(r.Body).Decode(&body)
		dbInstance.UpdateHeartbeat(body.Name, time.Now(), false)
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
