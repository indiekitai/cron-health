package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/indiekitai/cron-health/internal/config"
	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

type Server struct {
	db     *db.DB
	cfg    *config.Config
	port   int
	server *http.Server
}

func New(database *db.DB, cfg *config.Config, port int) *Server {
	return &Server{
		db:   database,
		cfg:  cfg,
		port: port,
	}
}

func (s *Server) Start(daemon bool) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping/", s.handlePing)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/monitors", s.handleAPIMonitors)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start background status checker
	ctx, cancel := context.WithCancel(context.Background())
	go s.statusChecker(ctx)

	if daemon {
		// Daemonize - fork to background
		log.Printf("cron-health server starting on port %d (daemon mode)", s.port)

		// Write PID file
		pidFile, err := config.GetConfigDir()
		if err == nil {
			pidPath := pidFile + "/server.pid"
			os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
		}

		go func() {
			if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		}()

		// Wait for shutdown signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		cancel()
		log.Println("Shutting down server...")
		return s.server.Shutdown(context.Background())
	}

	// Foreground mode
	log.Printf("cron-health server listening on port %d", s.port)
	log.Printf("Endpoints:")
	log.Printf("  GET /ping/<name>       - Record successful ping")
	log.Printf("  GET /ping/<name>/fail  - Record failed ping")
	log.Printf("  GET /ping/<name>/start - Record job started")
	log.Printf("  GET /health            - Health check")

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
		log.Println("Shutting down server...")
		s.server.Shutdown(context.Background())
	}()

	return s.server.ListenAndServe()
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ping/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Monitor name required", http.StatusBadRequest)
		return
	}

	name := parts[0]
	pingType := "success"

	if len(parts) > 1 {
		switch parts[1] {
		case "fail":
			pingType = "fail"
		case "start":
			pingType = "start"
		}
	}

	m, err := s.db.GetMonitorByName(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Monitor '%s' not found", name), http.StatusNotFound)
		return
	}

	oldStatus := m.Status
	if err := s.db.RecordPing(m.ID, pingType); err != nil {
		http.Error(w, "Failed to record ping", http.StatusInternalServerError)
		return
	}

	// Check for status change notification
	if pingType == "success" && oldStatus != "OK" && s.cfg.WebhookURL != "" {
		for _, n := range s.cfg.NotifyOn {
			if n == "recovered" {
				go sendNotification(s.cfg.WebhookURL, name, oldStatus, "OK")
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")

	log.Printf("Ping received: %s (%s)", name, pingType)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

func (s *Server) handleAPIMonitors(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.db.ListMonitors()
	if err != nil {
		http.Error(w, "Failed to list monitors", http.StatusInternalServerError)
		return
	}

	// Update statuses
	for _, m := range monitors {
		m.Status = monitor.CalculateStatus(m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(monitors)
}

func (s *Server) statusChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := monitor.UpdateAllStatuses(s.db, s.cfg); err != nil {
				log.Printf("Error updating statuses: %v", err)
			}
		}
	}
}

func sendNotification(webhookURL, monitorName, oldStatus, newStatus string) {
	payload := map[string]string{
		"monitor":    monitorName,
		"old_status": oldStatus,
		"new_status": newStatus,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", webhookURL, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Webhook failed: %v", err)
		return
	}
	resp.Body.Close()
}
