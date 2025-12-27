package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/db"
)

func TestDeleteHeartbeatEndpoint_RemovesDevice(t *testing.T) {
    dbPath := "test-integ-delete.db"
    var err error
    dbInstance, err = db.Open(dbPath)
    if err != nil {
        t.Fatalf("failed to open db: %v", err)
    }
    defer func() {
        _ = dbInstance.Close()
        _ = os.Remove(dbPath)
    }()

    // Insert two heartbeats
    if err := dbInstance.UpdateHeartbeat("clientA", time.Now(), false); err != nil {
        t.Fatalf("failed to insert clientA: %v", err)
    }
    if err := dbInstance.UpdateHeartbeat("clientB", time.Now(), false); err != nil {
        t.Fatalf("failed to insert clientB: %v", err)
    }

    mux := http.NewServeMux()
    // GET /heartbeats
    mux.HandleFunc("/heartbeats", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        heartbeats, err := dbInstance.GetAllHeartbeats()
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(heartbeats)
    })
    // DELETE /heartbeats/{name}
    mux.HandleFunc("/heartbeats/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodDelete {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        name := r.URL.Path[len("/heartbeats/"):]
        if name == "" {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte("Missing device name"))
            return
        }
        if err := dbInstance.Delete(name); err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("OK"))
    })

    ts := httptest.NewServer(mux)
    defer ts.Close()

    // Delete clientA
    req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/heartbeats/clientA", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("DELETE request failed: %v", err)
    }
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }

    // Verify clientA is gone and clientB remains
    resp2, err := http.Get(ts.URL + "/heartbeats")
    if err != nil {
        t.Fatalf("GET failed: %v", err)
    }
    defer resp2.Body.Close()
    var heartbeats map[string]db.ClientHeartbeat
    if err := json.NewDecoder(resp2.Body).Decode(&heartbeats); err != nil {
        t.Fatalf("decode failed: %v", err)
    }
    if _, ok := heartbeats["clientA"]; ok {
        t.Error("clientA should have been deleted but is still present")
    }
    if _, ok := heartbeats["clientB"]; !ok {
        t.Error("clientB should still be present but is missing")
    }
}

func TestDeleteHeartbeatEndpoint_BadRequest(t *testing.T) {
    dbPath := "test-integ-delete-bad.db"
    var err error
    dbInstance, err = db.Open(dbPath)
    if err != nil {
        t.Fatalf("failed to open db: %v", err)
    }
    defer func() {
        _ = dbInstance.Close()
        _ = os.Remove(dbPath)
    }()

    mux := http.NewServeMux()
    mux.HandleFunc("/heartbeats/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodDelete {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        name := r.URL.Path[len("/heartbeats/"):]
        if name == "" {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte("Missing device name"))
            return
        }
        if err := dbInstance.Delete(name); err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("OK"))
    })

    ts := httptest.NewServer(mux)
    defer ts.Close()

    // Call DELETE without a name
    req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/heartbeats/", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("DELETE request failed: %v", err)
    }
    if resp.StatusCode != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", resp.StatusCode)
    }
}
