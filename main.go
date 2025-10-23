package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github/bherbruck/mqtt-server/hooks/auth"
	"github/bherbruck/mqtt-server/hooks/metrics"
	"github/bherbruck/mqtt-server/hooks/retained"
	"github/bherbruck/mqtt-server/hooks/tracking"
	"github/bherbruck/mqtt-server/internal/api"
	"github/bherbruck/mqtt-server/internal/mqtt"
	"github/bherbruck/mqtt-server/internal/storage"
	"github/bherbruck/mqtt-server/web"
)

func main() {
	// Set up structured logging
	setupLogging()

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

	slog.Info("Starting MQTT Server")

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
	slog.Info("Connecting to database", "type", dbConfig.Type)
	db, err := storage.Open(dbConfig)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
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
		slog.Error("Failed to add auth hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Authentication hook registered")

	// Add ACL hook
	aclHook := auth.NewACLHook(db)
	if err := mqttServer.AddACLHook(aclHook); err != nil {
		slog.Error("Failed to add ACL hook", "error", err)
		os.Exit(1)
	}
	slog.Info("ACL hook registered")

	// Add metrics tracking hook with Prometheus
	promMetrics := mqtt.NewPrometheusMetrics()
	metricsHook := metrics.NewMetricsHook(promMetrics)
	if err := mqttServer.AddHook(metricsHook, nil); err != nil {
		slog.Error("Failed to add metrics hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Metrics hook registered")

	// Add retained message persistence hook
	// The hook will automatically load retained messages on startup via StoredRetainedMessages()
	retainedHook := retained.NewRetainedHook(db)
	if err := mqttServer.AddHook(retainedHook, nil); err != nil {
		slog.Error("Failed to add retained hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Retained message hook registered")

	// Add client tracking hook
	trackingHook := tracking.NewTrackingHook(db)
	if err := mqttServer.AddHook(trackingHook, nil); err != nil {
		slog.Error("Failed to add tracking hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Client tracking hook registered")

	// Start MQTT server in a goroutine
	go func() {
		if err := mqttServer.Start(); err != nil {
			slog.Error("Failed to start MQTT server", "error", err)
			os.Exit(1)
		}
	}()

	// Start HTTP API server in a goroutine
	apiServer := api.NewServer(*httpAddr, db, mqttServer, web.FS)
	go func() {
		if err := apiServer.Start(); err != nil {
			slog.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("===========================================")
	slog.Info("MQTT Server is running")
	slog.Info("  MQTT TCP", "address", *mqttTCP)
	slog.Info("  MQTT WebSocket", "address", *mqttWS)
	slog.Info("  HTTP API", "address", *httpAddr)
	if dbConfig.Type == "sqlite" {
		slog.Info("  Database", "type", dbConfig.Type, "path", dbConfig.FilePath)
	} else {
		slog.Info("  Database", "type", dbConfig.Type, "host", dbConfig.Host, "port", dbConfig.Port, "database", dbConfig.DBName)
	}
	slog.Info("===========================================")
	slog.Info("Default credentials: admin / admin")
	slog.Info("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down")
	mqttServer.Close()
	slog.Info("Server stopped")
}

// setupLogging configures slog based on environment variables
func setupLogging() {
	// Get log level from environment (default: info)
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info", "":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Get log format from environment (default: text)
	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch logFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text", "":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
