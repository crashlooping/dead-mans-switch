package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/config"
	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/db"
	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/notify"
)

var (
	lastHeartbeat time.Time
	dbInstance    *db.DB
	clientState   = make(map[string]bool) // true = missing, false = ok
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
					n.Notify("Dead Man's Switch Triggered", msg)
				}
				dbInstance.SetMissing(name, true)
			} else if !missed && ch.Missing {
				msg := "Heartbeat received again from client: " + name
				for _, n := range notifiers {
					n.Notify("Dead Man's Switch Recovery", msg)
				}
				dbInstance.SetMissing(name, false)
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
	defer dbInstance.Close()
	lastHeartbeat = time.Now()
	notifiers := setupNotifiers(cfg)
	go monitor(cfg, notifiers)
	os.Exit(runServer(cfg, notifiers))
}

func runServer(cfg *config.Config, notifiers []notify.Notifier) int {
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
			w.Write([]byte("Missing or invalid 'name' in body"))
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
		}
		if wasMissing {
			msg := "Heartbeat received again from client: " + body.Name
			for _, n := range notifiers {
				n.Notify("Dead Man's Switch Recovery", msg)
			}
			dbInstance.SetMissing(body.Name, false)
		}
		lastHeartbeat = now
		clientState[body.Name] = false // mark as healthy on any heartbeat
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		heartbeats, err := dbInstance.GetAllHeartbeats()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("DB error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(heartbeats)
	})

	// Web frontend: serve static index.html
	http.HandleFunc("/web", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

	http.HandleFunc("/web/devices", func(w http.ResponseWriter, r *http.Request) {
		heartbeats, err := dbInstance.GetAllHeartbeats()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("DB error"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<table>
			<thead><tr><th>Device</th><th>Last Seen</th><th>Missing</th></tr></thead><tbody>`))
		for name, ch := range heartbeats {
			w.Write([]byte("<tr>"))
			w.Write([]byte("<td>" + name + "</td>"))
			w.Write([]byte("<td>" + ch.Timestamp.Format("2006-01-02 15:04:05") + "</td>"))
			if ch.Missing {
				w.Write([]byte("<td style='color:red'>Yes</td>"))
			} else {
				w.Write([]byte("<td style='color:green'>No</td>"))
			}
			w.Write([]byte("</tr>"))
		}
		w.Write([]byte("</tbody></table>"))
	})

	http.HandleFunc("/web/configured-notifications", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<ul>`))
		for _, ch := range cfg.NotificationChannels {
			w.Write([]byte("<li><b>" + ch.Type + "</b>"))
			if len(ch.Properties) > 0 {
				w.Write([]byte(": <code>"))
				for k, v := range ch.Properties {
					if k == "bot_token" || k == "smtp_pass" || k == "smtp_user" || k == "smtp_from" {
						w.Write([]byte(k + "=***; "))
					} else {
						w.Write([]byte(k + "=" + v + "; "))
					}
				}
				w.Write([]byte("</code>"))
			}
			w.Write([]byte("</li>"))
		}
		w.Write([]byte(`</ul>`))
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
	server.Close()
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
