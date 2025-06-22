package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/config"
	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/db"
	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/notify"
)

var (
	dbInstance  *db.DB
	clientState = make(map[string]bool) // true = missing, false = ok
	sseClients  = make(map[chan string]struct{})
	sseMu       sync.Mutex
)

func monitor(cfg *config.Config, notifiers []notify.Notifier) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for range ticker.C {
		heartbeats, err := dbInstance.GetAllHeartbeats()
		if err != nil {
			log.Printf("DB error: %v", err)
			continue
		}
		for name, ch := range heartbeats {
			missed := time.Since(ch.Timestamp) > cfg.Timeout()
			duration := time.Since(ch.Timestamp).Round(time.Second)
			durStr := formatDuration(duration)
			if missed && !ch.Missing {
				msg := "No heartbeat received in time from client: " + name + ". Last update was " + durStr + " ago."
				for _, n := range notifiers {
					if err := n.Notify("Dead Man's Switch Triggered", msg); err != nil {
						log.Printf("Notify error: %v", err)
					}
				}
				if err := dbInstance.SetMissing(name, true); err != nil {
					log.Printf("SetMissing error: %v", err)
				}
				broadcastDeviceTable() // update SSE clients on timeout
			} else if !missed && ch.Missing {
				msg := "Heartbeat received again from client: " + name
				for _, n := range notifiers {
					if err := n.Notify("Dead Man's Switch Recovery", msg); err != nil {
						log.Printf("Notify error: %v", err)
					}
				}
				if err := dbInstance.SetMissing(name, false); err != nil {
					log.Printf("SetMissing error: %v", err)
				}
				broadcastDeviceTable() // update SSE clients on recovery
			}
		}
	}
}

func setupNotifiers(cfg *config.Config) []notify.Notifier {
	var result []notify.Notifier
	for _, ch := range cfg.NotificationChannels {
		n := notify.CreateNotifier(ch.Type, ch.Properties)
		if n != nil {
			result = append(result, n)
		}
	}
	return result
}

func broadcastDeviceTable() {
	heartbeats, err := dbInstance.GetAllHeartbeats()
	if err != nil {
		return
	}
	// Sort device names
	var names []string
	for name := range heartbeats {
		names = append(names, name)
	}
	sort.Strings(names)
	html := `<table><thead><tr><th>Device</th><th>Last Seen</th><th>Missing</th></tr></thead><tbody>`
	for _, name := range names {
		ch := heartbeats[name]
		html += "<tr>"
		html += "<td>" + name + "</td>"
		html += `<td data-utc='` + ch.Timestamp.UTC().Format(time.RFC3339) + `'>` + ch.Timestamp.UTC().Format(time.RFC3339) + "</td>"
		if ch.Missing {
			html += "<td style='color:red'>Yes</td>"
		} else {
			html += "<td style='color:green'>No</td>"
		}
		html += "</tr>"
	}
	html += "</tbody></table>"

	sseMu.Lock()
	for ch := range sseClients {
		select {
		case ch <- html:
		default:
		}
	}
	sseMu.Unlock()
}

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded: listen_addr=%s, timeout_seconds=%d, notification_channels=%v", cfg.ListenAddr, cfg.TimeoutSeconds, cfg.NotificationChannels)
	dbPath := "data/heartbeats.db"
	if os.Getenv("HEARTBEAT_DB_PATH") != "" {
		dbPath = os.Getenv("HEARTBEAT_DB_PATH")
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	dbInstance, err = db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer func() {
		if err := dbInstance.Close(); err != nil {
			log.Printf("DB close error: %v", err)
		}
	}()
	notifiers := setupNotifiers(cfg)
	go monitor(cfg, notifiers)
	os.Exit(runServer(cfg, notifiers))
}

func runServer(cfg *config.Config, notifiers []notify.Notifier) int {
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ch := make(chan string, 10)
		sseMu.Lock()
		sseClients[ch] = struct{}{}
		sseMu.Unlock()
		defer func() {
			sseMu.Lock()
			delete(sseClients, ch)
			sseMu.Unlock()
			close(ch)
		}()
		// Send initial table
		heartbeats, _ := dbInstance.GetAllHeartbeats()
		var names []string
		for name := range heartbeats {
			names = append(names, name)
		}
		sort.Strings(names)
		html := `<table><thead><tr><th>Device</th><th>Last Seen</th><th>Missing</th></tr></thead><tbody>`
		for _, name := range names {
			chb := heartbeats[name]
			html += "<tr>"
			html += "<td>" + name + "</td>"
			html += `<td data-utc='` + chb.Timestamp.UTC().Format(time.RFC3339) + `'>` + chb.Timestamp.UTC().Format(time.RFC3339) + "</td>"
			if chb.Missing {
				html += "<td style='color:red'>Yes</td>"
			} else {
				html += "<td style='color:green'>No</td>"
			}
			html += "</tr>"
		}
		html += "</tbody></table>"
		if _, err := fmt.Fprintf(w, "data: %s\n\n", html); err != nil {
			log.Printf("Fprintf error: %v", err)
		}
		w.(http.Flusher).Flush()
		for {
			select {
			case msg := <-ch:
				if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
					log.Printf("Fprintf error: %v", err)
				}
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		type req struct {
			Name string `json:"name"`
		}
		var body req
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil || body.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Missing or invalid 'name' in body"))
			return
		}
		log.Printf("Received heartbeat from client: %s", body.Name)
		now := time.Now()
		// Check if client was missing before updating
		heartbeats, _ := dbInstance.GetAllHeartbeats()
		wasMissing := false
		if ch, ok := heartbeats[body.Name]; ok {
			wasMissing = ch.Missing
		}
		err = dbInstance.UpdateHeartbeat(body.Name, now, false)
		if err != nil {
			log.Printf("DB update error for %s: %v", body.Name, err)
		} else {
			log.Printf("Stored to DB: {name: %s, timestamp: %s}", body.Name, now.Format(time.RFC3339))
			broadcastDeviceTable()
		}
		if wasMissing {
			msg := "Heartbeat received again from client: " + body.Name
			for _, n := range notifiers {
				if err := n.Notify("Dead Man's Switch Recovery", msg); err != nil {
					log.Printf("Notify error: %v", err)
				}
			}
			if err := dbInstance.SetMissing(body.Name, false); err != nil {
				log.Printf("SetMissing error: %v", err)
			}
		}
		clientState[body.Name] = false // mark as healthy on any heartbeat
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	http.HandleFunc("/heartbeats", func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("Encode error: %v", err)
		}
	})

	// Web frontend: serve static index.html
	http.HandleFunc("/web", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

	http.HandleFunc("/web/devices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<div id='device-table'></div>`)) // placeholder, SSE will update
	})

	http.HandleFunc("/web/configured-notifications", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<ul>`))
		for _, ch := range cfg.NotificationChannels {
			_, _ = w.Write([]byte("<li><b>" + ch.Type + "</b>"))
			if len(ch.Properties) > 0 {
				_, _ = w.Write([]byte(": <code>"))
				for k, v := range ch.Properties {
					if k == "bot_token" || k == "smtp_pass" || k == "smtp_user" || k == "smtp_from" {
						_, _ = w.Write([]byte(k + "=***; "))
					} else {
						_, _ = w.Write([]byte(k + "=" + v + "; "))
					}
				}
				_, _ = w.Write([]byte("</code>"))
			}
			_, _ = w.Write([]byte("</li>"))
		}
		_, _ = w.Write([]byte(`</ul>`))
	})

	http.HandleFunc("/up", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web", http.StatusFound)
	})
	server := &http.Server{Addr: cfg.ListenAddr}
	go func() {
		log.Printf("Starting server on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	if err := server.Close(); err != nil {
		log.Printf("server close error: %v", err)
	}
	return 0
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
