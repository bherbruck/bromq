package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bherbruck/configlib"
	"github/bherbruck/bromq/hooks/auth"
	"github/bherbruck/bromq/hooks/bridge"
	"github/bherbruck/bromq/hooks/metrics"
	"github/bherbruck/bromq/hooks/retained"
	scripthook "github/bherbruck/bromq/hooks/script"
	"github/bherbruck/bromq/hooks/tracking"
	"github/bherbruck/bromq/internal/api"
	"github/bherbruck/bromq/internal/appconfig"
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
	// Set up basic logging (will be reconfigured after parsing)
	setupBasicLogging()

	// Parse configuration from env vars, CLI flags, and defaults
	var cfg appconfig.Config
	if err := configlib.Parse(&cfg); err != nil {
		slog.Error("Failed to parse configuration", "error", err)
		os.Exit(1)
	}

	// Reconfigure logging with user preferences
	setupLogging(cfg.Logging.Level, cfg.Logging.Format)

	// Handle version flag
	if cfg.Version {
		fmt.Printf("BroMQ version %s\n", version)
		os.Exit(0)
	}

	slog.Info("Starting BroMQ", "version", version)

	// Initialize database
	slog.Info("Connecting to database", "type", cfg.Database.Type)
	db, err := storage.Open(&cfg.Database)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// Create default admin user if not exists (uses config from env vars, CLI flags, or defaults)
	if err := db.CreateDefaultAdmin(cfg.Admin.Username, cfg.Admin.Password); err != nil {
		slog.Warn("Failed to create default admin", "error", err)
	}

	// Load and provision configuration if provided
	if cfg.ConfigFile != "" {
		slog.Info("Loading configuration file", "path", cfg.ConfigFile)
		provCfg, err := config.Load(cfg.ConfigFile)
		if err != nil {
			slog.Error("Failed to load configuration file", "error", err)
			os.Exit(1)
		}

		if err := provisioning.Provision(db, provCfg); err != nil {
			slog.Error("Failed to provision configuration", "error", err)
			os.Exit(1)
		}
	}

	// Create MQTT server
	if cfg.MQTT.AllowAnonymous {
		slog.Warn("Anonymous MQTT connections are ENABLED - this is insecure for production use")
	} else {
		slog.Info("Anonymous MQTT connections are DISABLED (secure default)")
	}

	mqttServer := mqtt.New(&cfg.MQTT)

	// Add metrics tracking hook with Prometheus (create first so we can pass to other hooks)
	promMetrics := mqtt.NewPrometheusMetrics()
	metricsHook := metrics.NewMetricsHook(promMetrics)
	if err := mqttServer.AddHook(metricsHook, nil); err != nil {
		slog.Error("Failed to add metrics hook", "error", err)
		os.Exit(1)
	}
	slog.Info("Metrics hook registered")

	// Add authentication hook with metrics
	authHook := auth.NewAuthHook(db, cfg.MQTT.AllowAnonymous)
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

	// Start HTTP API server in a goroutine
	apiServer := api.NewServer(cfg.API.HTTPAddr, db, mqttServer, web.FS, scriptEngine, &cfg.API)
	go func() {
		if err := apiServer.Start(); err != nil {
			slog.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("===========================================")
	slog.Info("BroMQ is running")
	slog.Info("  MQTT TCP", "address", cfg.MQTT.TCPAddr)
	slog.Info("  MQTT WebSocket", "address", cfg.MQTT.WSAddr)
	slog.Info("  HTTP API", "address", cfg.API.HTTPAddr)
	if cfg.Database.Type == "sqlite" {
		slog.Info("  Database", "type", cfg.Database.Type, "path", cfg.Database.FilePath)
	} else {
		slog.Info("  Database", "type", cfg.Database.Type, "host", cfg.Database.Host, "port", cfg.Database.Port, "database", cfg.Database.DBName)
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

// setupBasicLogging configures a basic logger before config parsing
// This ensures we can log config parsing errors
func setupBasicLogging() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
}

// setupLogging reconfigures slog with user preferences from config
func setupLogging(logLevel, logFormat string) {
	// Parse log level
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Parse log format
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(logFormat) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
