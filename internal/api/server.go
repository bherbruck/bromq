package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"github/bherbruck/bromq/internal/api/swagger"
	"github/bherbruck/bromq/internal/mqtt"
	"github/bherbruck/bromq/internal/script"
	"github/bherbruck/bromq/internal/storage"
)

// Server represents the HTTP API server
type Server struct {
	handler *Handler
	config  *Config
	addr    string
	webFS   fs.FS
}

// NewServer creates a new API server
func NewServer(addr string, db *storage.DB, mqttServer *mqtt.Server, webFS fs.FS, scriptEngine *script.Engine, config *Config) *Server {
	return &Server{
		handler: NewHandler(db, mqttServer, scriptEngine, config),
		config:  config,
		addr:    addr,
		webFS:   webFS,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Create authentication middleware with config
	authMiddleware := NewAuthMiddleware(s.config)

	// API routes
	apiMux := http.NewServeMux()

	// Public routes
	apiMux.HandleFunc("POST /auth/login", s.handler.Login)

	// Password change endpoint (any authenticated user can change their own password)
	apiMux.Handle("PUT /auth/change-password", authMiddleware(http.HandlerFunc(s.handler.ChangePassword)))

	// === Dashboard User Management ===
	// List dashboard users - any authenticated user can view
	apiMux.Handle("GET /dashboard/users", authMiddleware(http.HandlerFunc(s.handler.ListDashboardUsers)))
	apiMux.Handle("GET /dashboard/users/{id}", authMiddleware(http.HandlerFunc(s.handler.GetDashboardUser)))
	// Manage dashboard users - admin only
	apiMux.Handle("POST /dashboard/users", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateDashboardUser))))
	apiMux.Handle("PUT /dashboard/users/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateDashboardUser))))
	apiMux.Handle("PUT /dashboard/users/{id}/password", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateDashboardUserPassword))))
	apiMux.Handle("DELETE /dashboard/users/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteDashboardUser))))

	// === MQTT Management ===
	// View MQTT resources - any authenticated user can view
	apiMux.Handle("GET /mqtt/users", authMiddleware(http.HandlerFunc(s.handler.ListMQTTUsers)))
	apiMux.Handle("GET /mqtt/users/{id}", authMiddleware(http.HandlerFunc(s.handler.GetMQTTUser)))
	apiMux.Handle("GET /mqtt/clients", authMiddleware(http.HandlerFunc(s.handler.ListMQTTClients)))
	apiMux.Handle("GET /mqtt/clients/{client_id}", authMiddleware(http.HandlerFunc(s.handler.GetMQTTClientDetails)))
	apiMux.Handle("GET /acl", authMiddleware(http.HandlerFunc(s.handler.ListACL)))

	// Manage MQTT users - admin only
	apiMux.Handle("POST /mqtt/users", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateMQTTUser))))
	apiMux.Handle("PUT /mqtt/users/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateMQTTUser))))
	apiMux.Handle("PUT /mqtt/users/{id}/password", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateMQTTUserPassword))))
	apiMux.Handle("DELETE /mqtt/users/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteMQTTUser))))

	// Manage MQTT clients - admin only
	apiMux.Handle("PUT /mqtt/clients/{client_id}/metadata", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateMQTTClientMetadata))))
	apiMux.Handle("DELETE /mqtt/clients/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteMQTTClient))))

	// Manage ACL rules - admin only
	apiMux.Handle("POST /acl", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateACL))))
	apiMux.Handle("PUT /acl/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateACL))))
	apiMux.Handle("DELETE /acl/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteACL))))

	// === Bridge Management ===
	// View bridges - any authenticated user can view
	apiMux.Handle("GET /bridges", authMiddleware(http.HandlerFunc(s.handler.ListBridges)))
	apiMux.Handle("GET /bridges/{id}", authMiddleware(http.HandlerFunc(s.handler.GetBridge)))

	// Manage bridges - admin only
	apiMux.Handle("POST /bridges", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateBridge))))
	apiMux.Handle("PUT /bridges/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateBridge))))
	apiMux.Handle("DELETE /bridges/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteBridge))))

	// === Script Management ===
	// View scripts and logs - any authenticated user can view
	apiMux.Handle("GET /scripts", authMiddleware(http.HandlerFunc(s.handler.ListScripts)))
	apiMux.Handle("GET /scripts/{id}", authMiddleware(http.HandlerFunc(s.handler.GetScript)))
	apiMux.Handle("GET /scripts/{id}/logs", authMiddleware(http.HandlerFunc(s.handler.GetScriptLogs)))
	apiMux.Handle("GET /scripts/{id}/state", authMiddleware(http.HandlerFunc(s.handler.GetScriptState)))

	// Manage scripts - admin only
	apiMux.Handle("POST /scripts", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateScript))))
	apiMux.Handle("PUT /scripts/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateScript))))
	apiMux.Handle("DELETE /scripts/{id}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteScript))))
	apiMux.Handle("POST /scripts/{id}/enable", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.EnableScript))))
	apiMux.Handle("POST /scripts/test", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.TestScript))))
	apiMux.Handle("DELETE /scripts/{id}/logs", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.ClearScriptLogs))))
	apiMux.Handle("DELETE /scripts/{id}/state/{key}", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteScriptStateKey))))

	// Legacy/deprecated clients endpoint (for backward compatibility)
	apiMux.Handle("GET /clients", authMiddleware(http.HandlerFunc(s.handler.ListClients)))
	apiMux.Handle("GET /clients/{id}", authMiddleware(http.HandlerFunc(s.handler.GetClientDetails)))
	apiMux.Handle("POST /clients/{id}/disconnect", authMiddleware(AdminOnly(http.HandlerFunc(s.handler.DisconnectClient))))

	// Metrics - any authenticated user can view
	apiMux.Handle("GET /metrics", authMiddleware(http.HandlerFunc(s.handler.GetMetrics)))

	// Mount API under /api
	mux.Handle("/api/", http.StripPrefix("/api", apiMux))

	// Health check endpoint (no auth required)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// Prometheus metrics endpoint (no auth required)
	mux.Handle("/metrics", promhttp.Handler())

	// Swagger spec endpoint (no auth required)
	mux.HandleFunc("GET /swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(swagger.SwaggerJSON)
	})

	// Swagger UI endpoint (no auth required)
	mux.HandleFunc("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Serve frontend (embedded)
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		mux.Handle("/", spaHandler(s.webFS, fileServer))
	} else {
		slog.Warn("Frontend not available")
	}

	// Apply middleware
	handler := LoggingMiddleware(CORSMiddleware(mux))

	// Create server with timeouts to prevent resource exhaustion
	server := &http.Server{
		Addr:           s.addr,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	slog.Info("HTTP API server started", "address", s.addr)
	return server.ListenAndServe()
}

// spaHandler serves the Single Page Application with fallback to index.html
func spaHandler(fsys fs.FS, fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to open the file
		path := r.URL.Path
		if path == "" || path == "/" {
			path = "index.html"
		} else {
			path = path[1:] // Remove leading slash
		}

		if _, err := fs.Stat(fsys, path); err != nil {
			// File not found, serve index.html (SPA fallback)
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	}
}
