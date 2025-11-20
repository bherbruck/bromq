package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github/bherbruck/bromq/hooks/auth"
	"github/bherbruck/bromq/hooks/bridge"
	"github/bherbruck/bromq/hooks/metrics"
	"github/bherbruck/bromq/hooks/retained"
	scripthook "github/bherbruck/bromq/hooks/script"
	"github/bherbruck/bromq/hooks/tracking"
	"github/bherbruck/bromq/internal/api"
	"github/bherbruck/bromq/internal/config"
	"github/bherbruck/bromq/internal/mqtt"
	"github/bherbruck/bromq/internal/provisioning"
	"github/bherbruck/bromq/internal/script"
	"github/bherbruck/bromq/internal/storage"
	"github/bherbruck/bromq/web"
)

// version is set via ldflags during build
var version = "dev"

func main() {
	// Set up structured logging
	setupLogging()

	// Parse command line flags
	showVersion := flag.Bool("version", false, "Show version and exit")
	dbType := flag.String("db-type", "", "Database type (sqlite, postgres, mysql). Defaults to DB_TYPE env var or 'sqlite'")
	dbPath := flag.String("db-path", "", "SQLite database file path. Defaults to DB_PATH env var or 'bromq.db'")
	dbHost := flag.String("db-host", "", "Database host (postgres/mysql). Defaults to DB_HOST env var or 'localhost'")
	dbPort := flag.Int("db-port", 0, "Database port (postgres/mysql). Defaults to DB_PORT env var or default port")
	dbUser := flag.String("db-user", "", "Database user (postgres/mysql). Defaults to DB_USER env var or 'mqtt'")
	dbPassword := flag.String("db-password", "", "Database password (postgres/mysql). Defaults to DB_PASSWORD env var")
	dbName := flag.String("db-name", "", "Database name (postgres/mysql). Defaults to DB_NAME env var or 'mqtt'")
	dbSSLMode := flag.String("db-sslmode", "", "SSL mode for postgres (disable, require, verify-ca, verify-full). Defaults to DB_SSLMODE env var or 'disable'")
	configFile := flag.String("config", "", "Path to configuration file for provisioning MQTT users and ACL rules. Defaults to CONFIG_FILE env var")
	mqttTCP := flag.String("mqtt-tcp", ":1883", "MQTT TCP listener address")
	mqttWS := flag.String("mqtt-ws", ":8883", "MQTT WebSocket listener address")
	httpAddr := flag.String("http", ":8080", "HTTP API server address")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("BroMQ version %s\n", version)
		os.Exit(0)
	}

	slog.Info("Starting BroMQ", "version", version)

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
	defer func() { _ = db.Close() }()

	// Load and provision configuration if provided
	configPath := *configFile
	if configPath == "" {
		configPath = os.Getenv("CONFIG_FILE")
	}
	if configPath != "" {
		slog.Info("Loading configuration file", "path", configPath)
		cfg, err := config.Load(configPath)
		if err != nil {
			slog.Error("Failed to load configuration file", "error", err)
			os.Exit(1)
		}

		if err := provisioning.Provision(db, cfg); err != nil {
			slog.Error("Failed to provision configuration", "error", err)
			os.Exit(1)
		}
	}

	// Create MQTT server
	allowAnonymous := os.Getenv("MQTT_ALLOW_ANONYMOUS") == "true"
	if allowAnonymous {
		slog.Warn("Anonymous MQTT connections are ENABLED - this is insecure for production use")
	} else {
		slog.Info("Anonymous MQTT connections are DISABLED (secure default)")
	}

	mqttConfig := &mqtt.Config{
		TCPAddr:         *mqttTCP,
		WSAddr:          *mqttWS,
		EnableTLS:       false,
		MaxClients:      0,
		RetainAvailable: true,
		AllowAnonymous:  allowAnonymous,
	}
	mqttServer := mqtt.New(mqttConfig)

	// Add metrics tracking hook with Prometheus (create first so we can pass to other hooks)
	promMetrics := mqtt.NewPrometheusMetrics()
	metricsHook := metrics.NewMetricsHook(promMetrics)
	if err := mqttServer.AddHook(metricsHook, nil); err != nil {
		slog.Error("Failed to add metrics hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Metrics hook registered")

	// Add authentication hook with metrics
	authHook := auth.NewAuthHook(db, mqttConfig.AllowAnonymous)
	authHook.SetMetrics(promMetrics)
	if err := mqttServer.AddAuthHook(authHook); err != nil {
		slog.Error("Failed to add auth hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Authentication hook registered")

	// Add ACL hook with metrics
	aclHook := auth.NewACLHook(db)
	aclHook.SetMetrics(promMetrics)
	if err := mqttServer.AddACLHook(aclHook); err != nil {
		slog.Error("Failed to add ACL hook", "error", err)
		os.Exit(1)
	}
	slog.Info("ACL hook registered")

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

	// Initialize bridge manager and hook
	bridgeManager := bridge.NewManager(db, mqttServer.Server)
	bridgeHook := bridge.NewBridgeHook(bridgeManager)
	if err := mqttServer.AddHook(bridgeHook, nil); err != nil {
		slog.Error("Failed to add bridge hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Bridge hook registered")

	// Initialize script engine and hook
	scriptEngine := script.NewEngine(db, mqttServer.Server)
	scriptEngine.Start()
	scriptHookInstance := scripthook.NewScriptHook(scriptEngine)
	if err := mqttServer.AddHook(scriptHookInstance, nil); err != nil {
		slog.Error("Failed to add script hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Script hook registered")

	// Start MQTT server in a goroutine
	go func() {
		if err := mqttServer.Start(); err != nil {
			slog.Error("Failed to start MQTT server", "error", err)
			os.Exit(1)
		}
	}()

	// Start bridge connections after server is running
	if err := bridgeManager.Start(); err != nil {
		slog.Error("Failed to start bridge connections", "error", err)
		// Don't exit - bridges are optional, continue without them
	}

	// Load API configuration (JWT secret)
	apiConfig := api.LoadConfig()

	// Start HTTP API server in a goroutine
	apiServer := api.NewServer(*httpAddr, db, mqttServer, web.FS, scriptEngine, apiConfig)
	go func() {
		if err := apiServer.Start(); err != nil {
			slog.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("===========================================")
	slog.Info("BroMQ is running")
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

	slog.Info("Shutting down gracefully...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Stop MQTT server (no new connections)
	slog.Info("Stopping MQTT server...")
	if err := mqttServer.Close(); err != nil {
		slog.Error("Error closing MQTT server", "error", err)
	}

	// 2. Stop bridge connections
	slog.Info("Stopping bridges...")
	bridgeManager.Stop()

	// 3. Shutdown script engine (CRITICAL: final state flush)
	slog.Info("Shutting down script engine...")
	if err := scriptEngine.Shutdown(ctx); err != nil {
		slog.Error("Error shutting down script engine", "error", err)
	}

	// 4. Close database
	slog.Info("Closing database...")
	if err := db.Close(); err != nil {
		slog.Error("Error closing database", "error", err)
	}

	slog.Info("Shutdown complete")
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
