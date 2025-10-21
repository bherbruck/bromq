package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github/bherbruck/mqtt-server/hooks/auth"
	"github/bherbruck/mqtt-server/hooks/metrics"
	"github/bherbruck/mqtt-server/hooks/retained"
	"github/bherbruck/mqtt-server/internal/api"
	"github/bherbruck/mqtt-server/internal/mqtt"
	"github/bherbruck/mqtt-server/internal/storage"
)

//go:embed all:web/dist/client
var webFS embed.FS

func main() {
	// Parse command line flags
	dbPath := flag.String("db", "mqtt-server.db", "SQLite database file path")
	mqttTCP := flag.String("mqtt-tcp", ":1883", "MQTT TCP listener address")
	mqttWS := flag.String("mqtt-ws", ":8883", "MQTT WebSocket listener address")
	httpAddr := flag.String("http", ":8080", "HTTP API server address")
	flag.Parse()

	log.Println("Starting MQTT Server...")

	// Initialize database
	log.Printf("Opening database: %s", *dbPath)
	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create MQTT server
	mqttConfig := &mqtt.Config{
		TCPAddr:         *mqttTCP,
		WSAddr:          *mqttWS,
		EnableTLS:       false,
		MaxClients:      0,
		RetainAvailable: true,
	}
	mqttServer := mqtt.New(mqttConfig)

	// Add authentication hook
	authHook := auth.NewAuthHook(db)
	if err := mqttServer.AddAuthHook(authHook); err != nil {
		log.Fatalf("Failed to add auth hook: %v", err)
	}
	log.Println("Authentication hook registered")

	// Add ACL hook
	aclHook := auth.NewACLHook(db)
	if err := mqttServer.AddACLHook(aclHook); err != nil {
		log.Fatalf("Failed to add ACL hook: %v", err)
	}
	log.Println("ACL hook registered")

	// Add metrics tracking hook with Prometheus
	promMetrics := mqtt.NewPrometheusMetrics()
	metricsHook := metrics.NewMetricsHook(promMetrics)
	if err := mqttServer.AddHook(metricsHook, nil); err != nil {
		log.Fatalf("Failed to add metrics hook: %v", err)
	}
	log.Println("Metrics hook registered")

	// Add retained message persistence hook
	// The hook will automatically load retained messages on startup via StoredRetainedMessages()
	retainedHook := retained.NewRetainedHook(db)
	if err := mqttServer.AddHook(retainedHook, nil); err != nil {
		log.Fatalf("Failed to add retained hook: %v", err)
	}
	log.Println("Retained message hook registered")

	// Start MQTT server in a goroutine
	go func() {
		if err := mqttServer.Start(); err != nil {
			log.Fatalf("Failed to start MQTT server: %v", err)
		}
	}()

	// Start HTTP API server in a goroutine
	apiServer := api.NewServer(*httpAddr, db, mqttServer, webFS)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	log.Println("===========================================")
	log.Println("MQTT Server is running")
	log.Printf("  MQTT TCP:       %s", *mqttTCP)
	log.Printf("  MQTT WebSocket: %s", *mqttWS)
	log.Printf("  HTTP API:       %s", *httpAddr)
	log.Printf("  Database:       %s", *dbPath)
	log.Println("===========================================")
	log.Println("Default credentials: admin / admin")
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	mqttServer.Close()
	log.Println("Server stopped")
}
