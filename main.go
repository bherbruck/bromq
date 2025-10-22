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
	"github/bherbruck/mqtt-server/hooks/tracking"
	"github/bherbruck/mqtt-server/internal/api"
	"github/bherbruck/mqtt-server/internal/mqtt"
	"github/bherbruck/mqtt-server/internal/storage"
)

//go:embed all:web/dist/client
var webFS embed.FS

func main() {
	// Parse command line flags
	dbType := flag.String("db-type", "", "Database type (sqlite, postgres, mysql). Defaults to DB_TYPE env var or 'sqlite'")
	dbPath := flag.String("db-path", "", "SQLite database file path. Defaults to DB_PATH env var or 'mqtt-server.db'")
	dbHost := flag.String("db-host", "", "Database host (postgres/mysql). Defaults to DB_HOST env var or 'localhost'")
	dbPort := flag.Int("db-port", 0, "Database port (postgres/mysql). Defaults to DB_PORT env var or default port")
	dbUser := flag.String("db-user", "", "Database user (postgres/mysql). Defaults to DB_USER env var or 'mqtt'")
	dbPassword := flag.String("db-password", "", "Database password (postgres/mysql). Defaults to DB_PASSWORD env var")
	dbName := flag.String("db-name", "", "Database name (postgres/mysql). Defaults to DB_NAME env var or 'mqtt'")
	dbSSLMode := flag.String("db-sslmode", "", "SSL mode for postgres (disable, require, verify-ca, verify-full). Defaults to DB_SSLMODE env var or 'disable'")
	mqttTCP := flag.String("mqtt-tcp", ":1883", "MQTT TCP listener address")
	mqttWS := flag.String("mqtt-ws", ":8883", "MQTT WebSocket listener address")
	httpAddr := flag.String("http", ":8080", "HTTP API server address")
	flag.Parse()

	log.Println("Starting MQTT Server...")

	// Load database configuration from environment variables first
	dbConfig := storage.LoadConfigFromEnv()

	// Override with command-line flags if provided
	if *dbType != "" {
		dbConfig.Type = *dbType
	}
	if *dbPath != "" {
		dbConfig.FilePath = *dbPath
	}
	if *dbHost != "" {
		dbConfig.Host = *dbHost
	}
	if *dbPort != 0 {
		dbConfig.Port = *dbPort
	}
	if *dbUser != "" {
		dbConfig.User = *dbUser
	}
	if *dbPassword != "" {
		dbConfig.Password = *dbPassword
	}
	if *dbName != "" {
		dbConfig.DBName = *dbName
	}
	if *dbSSLMode != "" {
		dbConfig.SSLMode = *dbSSLMode
	}

	// Initialize database
	log.Printf("Connecting to database (type: %s)", dbConfig.Type)
	db, err := storage.Open(dbConfig)
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

	// Add client tracking hook
	trackingHook := tracking.NewTrackingHook(db)
	if err := mqttServer.AddHook(trackingHook, nil); err != nil {
		log.Fatalf("Failed to add tracking hook: %v", err)
	}
	log.Println("Client tracking hook registered")

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
	if dbConfig.Type == "sqlite" {
		log.Printf("  Database:       %s (%s)", dbConfig.Type, dbConfig.FilePath)
	} else {
		log.Printf("  Database:       %s (%s:%d/%s)", dbConfig.Type, dbConfig.Host, dbConfig.Port, dbConfig.DBName)
	}
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
