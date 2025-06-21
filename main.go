package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/config"
	"github.com/crashlooping/dead-mans-switch/dead-mans-switch/notify"
)

var (
	lastHeartbeat time.Time
)

func monitor(cfg *config.Config, notifiers []notify.Notifier) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for range ticker.C {
		if time.Since(lastHeartbeat) > cfg.Timeout() {
			for _, n := range notifiers {
				n.Notify("Dead Man's Switch Triggered", "No heartbeat received in time!")
			}
			lastHeartbeat = time.Now() // avoid spamming
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
	lastHeartbeat = time.Now()
	notifiers := setupNotifiers(cfg)
	go monitor(cfg, notifiers)
	os.Exit(runServer(cfg))
}

func runServer(cfg *config.Config) int {
	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		lastHeartbeat = time.Now()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
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
