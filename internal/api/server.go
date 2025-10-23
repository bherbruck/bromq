package api

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github/bherbruck/mqtt-server/internal/mqtt"
	"github/bherbruck/mqtt-server/internal/storage"
)

// Server represents the HTTP API server
type Server struct {
	handler *Handler
	addr    string
	webFS   fs.FS
}

// NewServer creates a new API server
func NewServer(addr string, db *storage.DB, mqttServer *mqtt.Server, webFS fs.FS) *Server {
	return &Server{
		handler: NewHandler(db, mqttServer),
		addr:    addr,
		webFS:   webFS,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	apiMux := http.NewServeMux()

	// Public routes
	apiMux.HandleFunc("POST /auth/login", s.handler.Login)

	// Password change endpoint (any authenticated user can change their own password)
	apiMux.Handle("PUT /auth/change-password", AuthMiddleware(http.HandlerFunc(s.handler.ChangePassword)))

	// === Dashboard User Management ===
	// List dashboard users - any authenticated user can view
	apiMux.Handle("GET /dashboard/users", AuthMiddleware(http.HandlerFunc(s.handler.ListDashboardUsers)))
	// Manage dashboard users - admin only
	apiMux.Handle("POST /dashboard/users", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateDashboardUser))))
	apiMux.Handle("PUT /dashboard/users/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateDashboardUser))))
	apiMux.Handle("PUT /dashboard/users/{id}/password", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateDashboardUserPassword))))
	apiMux.Handle("DELETE /dashboard/users/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteDashboardUser))))

	// === MQTT Management ===
	// View MQTT resources - any authenticated user can view
	apiMux.Handle("GET /mqtt/credentials", AuthMiddleware(http.HandlerFunc(s.handler.ListMQTTUsers)))
	apiMux.Handle("GET /mqtt/clients", AuthMiddleware(http.HandlerFunc(s.handler.ListMQTTClients)))
	apiMux.Handle("GET /mqtt/clients/{client_id}", AuthMiddleware(http.HandlerFunc(s.handler.GetMQTTClientDetails)))
	apiMux.Handle("GET /acl", AuthMiddleware(http.HandlerFunc(s.handler.ListACL)))

	// Manage MQTT users - admin only
	apiMux.Handle("POST /mqtt/credentials", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateMQTTUser))))
	apiMux.Handle("PUT /mqtt/credentials/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateMQTTUser))))
	apiMux.Handle("PUT /mqtt/credentials/{id}/password", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateMQTTUserPassword))))
	apiMux.Handle("DELETE /mqtt/credentials/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteMQTTUser))))

	// Manage MQTT clients - admin only
	apiMux.Handle("PUT /mqtt/clients/{client_id}/metadata", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateMQTTClientMetadata))))
	apiMux.Handle("DELETE /mqtt/clients/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteMQTTClient))))

	// Manage ACL rules - admin only
	apiMux.Handle("POST /acl", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateACL))))
	apiMux.Handle("PUT /acl/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateACL))))
	apiMux.Handle("DELETE /acl/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteACL))))

	// Legacy/deprecated clients endpoint (for backward compatibility)
	apiMux.Handle("GET /clients", AuthMiddleware(http.HandlerFunc(s.handler.ListClients)))
	apiMux.Handle("GET /clients/{id}", AuthMiddleware(http.HandlerFunc(s.handler.GetClientDetails)))
	apiMux.Handle("POST /clients/{id}/disconnect", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DisconnectClient))))

	// Metrics - any authenticated user can view
	apiMux.Handle("GET /metrics", AuthMiddleware(http.HandlerFunc(s.handler.GetMetrics)))

	// Mount API under /api
	mux.Handle("/api/", http.StripPrefix("/api", apiMux))

	// Health check endpoint (no auth required)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Prometheus metrics endpoint (no auth required)
	mux.Handle("/metrics", promhttp.Handler())

	// Serve frontend (embedded)
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		mux.Handle("/", spaHandler(s.webFS, fileServer))
	} else {
		log.Printf("Warning: frontend not available")
	}

	// Apply middleware
	handler := LoggingMiddleware(CORSMiddleware(mux))

	log.Printf("HTTP API server started on %s", s.addr)
	return http.ListenAndServe(s.addr, handler)
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
