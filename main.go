package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
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

var BuildTime = "dev"
var GitCommit = "none"

func monitor(cfg *config.Config, notifiers []notify.Notifier) {
	// Wait until the next minute boundary (0 seconds)
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(nextMinute))

	ticker := time.NewTicker(time.Minute)
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
				msg := cfg.NotificationMessages.Timeout
				if msg == "" {
					msg = "No heartbeat received in time from client: {{name}}. Last update was {{duration}} ago."
				}
				msg = strings.ReplaceAll(msg, "{{name}}", name)
				msg = strings.ReplaceAll(msg, "{{duration}}", durStr)
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
				msg := cfg.NotificationMessages.Recovery
				if msg == "" {
					msg = "Heartbeat received again from client: {{name}}"
				}
				msg = strings.ReplaceAll(msg, "{{name}}", name)
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
			html += `<td class='status status-yes'><span class='status-icon' aria-label='Missing' title='Missing'>` +
				// Not OK SVG (red)
				`<svg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke-width='1.5' stroke='#e53e3e' width='22' height='22'><path stroke-linecap='round' stroke-linejoin='round' d='M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z'/></svg>` +
				`</span> <span class='status-text'>yes</span></td>`
		} else {
			html += `<td class='status status-no'><span class='status-icon' aria-label='OK' title='OK'>` +
				// OK SVG (green)
				`<svg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke-width='1.5' stroke='#38a169' width='22' height='22'><path stroke-linecap='round' stroke-linejoin='round' d='M8.288 15.038a5.25 5.25 0 0 1 7.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 0 1 1.06 0Z'/></svg>` +
				`</span> <span class='status-text'>no</span></td>`
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

	// Validate timeout_seconds is at least 60
	if cfg.TimeoutSeconds < 60 {
		log.Fatalf("timeout_seconds must be at least 60 seconds, got %d", cfg.TimeoutSeconds)
	}

	// Create a masked copy of notification channels for logging
	maskedChannels := make([]config.NotificationChannel, len(cfg.NotificationChannels))
	for i, ch := range cfg.NotificationChannels {
		maskedChannels[i] = config.NotificationChannel{
			Type:       ch.Type,
			Properties: make(map[string]string),
		}
		for k, v := range ch.Properties {
			switch k {
			case "bot_token":
				if len(v) > 6 {
					maskedChannels[i].Properties[k] = v[:3] + "***" + v[len(v)-3:]
				} else {
					maskedChannels[i].Properties[k] = "***"
				}
			case "smtp_pass":
				maskedChannels[i].Properties[k] = "***"
			default:
				maskedChannels[i].Properties[k] = v
			}
		}
	}

	log.Printf("Config loaded: listen_addr=%s, timeout_seconds=%d, notification_channels=%v", cfg.ListenAddr, cfg.TimeoutSeconds, maskedChannels)

	// Log build metadata
	log.Printf("Build Time: %s, Git Commit: %s", BuildTime, GitCommit)

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
	basePath := os.Getenv("BASE_PATH") // e.g. "/dead-mans-switch"
	if basePath == "/" {
		basePath = ""
	}
	mux := http.NewServeMux()

	mux.HandleFunc(basePath+"/events", func(w http.ResponseWriter, r *http.Request) {
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
				html += `<td class='status status-yes'><span class='status-icon' aria-label='Missing' title='Missing'>` +
					// Not OK SVG (red)
					`<svg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke-width='1.5' stroke='#e53e3e' width='22' height='22'><path stroke-linecap='round' stroke-linejoin='round' d='M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z'/></svg>` +
					`</span> <span class='status-text'>yes</span></td>`
			} else {
				html += `<td class='status status-no'><span class='status-icon' aria-label='OK' title='OK'>` +
					// OK SVG (green)
					`<svg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke-width='1.5' stroke='#38a169' width='22' height='22'><path stroke-linecap='round' stroke-linejoin='round' d='M8.288 15.038a5.25 5.25 0 0 1 7.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 0 1 1.06 0Z'/></svg>` +
					`</span> <span class='status-text'>no</span></td>`
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

	mux.HandleFunc(basePath+"/heartbeat", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc(basePath+"/heartbeats", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc(basePath+"/web", func(w http.ResponseWriter, r *http.Request) {
		// Inject basePath, build metadata into the HTML as data attributes on <body>
		index, err := os.ReadFile("web/index.html")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("index.html not found"))
			return
		}
		// Insert data attributes
		html := strings.Replace(string(index), "<body>", fmt.Sprintf("<body data-base-path='%s' data-build-time='%s' data-git-commit='%s'>", basePath, BuildTime, GitCommit), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})

	mux.HandleFunc(basePath+"/web/devices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<div id='device-table'></div>`)) // placeholder, SSE will update
	})

	mux.HandleFunc(basePath+"/web/configured-notifications", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<ul>`))
		// Sort notification channels by type
		notifs := make([]config.NotificationChannel, len(cfg.NotificationChannels))
		copy(notifs, cfg.NotificationChannels)
		sort.Slice(notifs, func(i, j int) bool {
			return notifs[i].Type < notifs[j].Type
		})
		for _, ch := range notifs {
			_, _ = w.Write([]byte("<li><b>" + ch.Type + "</b>"))
			if len(ch.Properties) > 0 {
				_, _ = w.Write([]byte(": <code>"))
				// Sort property keys
				var keys []string
				for k := range ch.Properties {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					v := ch.Properties[k]
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

	mux.HandleFunc(basePath+"/up", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc(basePath+"/web/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// Try to read config.yaml and mask secrets
		content, err := os.ReadFile("config.yaml")
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if strings.Contains(line, "bot_token:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						value := strings.TrimSpace(parts[1])
						// Check if value is quoted and extract the actual token
						if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) && len(value) > 2 {
							token := value[1 : len(value)-1] // Remove quotes
							if len(token) > 6 {
								lines[i] = parts[0] + `: "` + token[:3] + "***" + token[len(token)-3:] + `"`
							} else {
								lines[i] = parts[0] + `: "***"`
							}
						} else if len(value) > 6 {
							lines[i] = parts[0] + ": " + value[:3] + "***" + value[len(value)-3:]
						} else {
							lines[i] = parts[0] + ": ***"
						}
					}
				} else if strings.Contains(line, "pass:") || strings.Contains(line, "token:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						lines[i] = parts[0] + ": ***"
					}
				}
			}
			masked := strings.Join(lines, "\n")
			if _, err := w.Write([]byte(masked)); err != nil {
				log.Printf("error writing masked config: %v", err)
			}
			return
		}
		// fallback: show as JSON with masked secrets
		masked := cfg
		for i := range masked.NotificationChannels {
			for k, v := range masked.NotificationChannels[i].Properties {
				if k == "bot_token" {
					if len(v) > 6 {
						masked.NotificationChannels[i].Properties[k] = v[:3] + "***" + v[len(v)-3:]
					} else {
						masked.NotificationChannels[i].Properties[k] = "***"
					}
				} else if strings.Contains(k, "pass") || strings.Contains(k, "token") || strings.Contains(k, "secret") {
					masked.NotificationChannels[i].Properties[k] = "***"
				}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(masked)
	})

	// Serve static files under /web if needed (e.g. /web/htmx.js)
	mux.Handle(basePath+"/web/", http.StripPrefix(basePath+"/web/", http.FileServer(http.Dir("web"))))

	mux.HandleFunc(basePath+"/", func(w http.ResponseWriter, r *http.Request) {
		// Only redirect if the path is exactly basePath or basePath+
		if r.URL.Path == basePath || r.URL.Path == basePath+"/" {
			http.Redirect(w, r, basePath+"/web", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not found"))
	})

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}
	go func() {
		log.Printf("Starting server on %s (basePath: %s)", cfg.ListenAddr, basePath)
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
