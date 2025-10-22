package api

import (
	"embed"
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
	webFS   embed.FS
}

// NewServer creates a new API server
func NewServer(addr string, db *storage.DB, mqttServer *mqtt.Server, webFS embed.FS) *Server {
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

	// Protected routes (require authentication)
	apiMux.Handle("GET /users", AuthMiddleware(http.HandlerFunc(s.handler.ListUsers)))
	apiMux.Handle("POST /users", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateUser))))
	apiMux.Handle("PUT /users/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.UpdateUser))))
	apiMux.Handle("DELETE /users/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteUser))))

	apiMux.Handle("GET /acl", AuthMiddleware(http.HandlerFunc(s.handler.ListACL)))
	apiMux.Handle("POST /acl", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.CreateACL))))
	apiMux.Handle("DELETE /acl/{id}", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DeleteACL))))

	apiMux.Handle("GET /clients", AuthMiddleware(http.HandlerFunc(s.handler.ListClients)))
	apiMux.Handle("GET /clients/{id}", AuthMiddleware(http.HandlerFunc(s.handler.GetClientDetails)))
	apiMux.Handle("POST /clients/{id}/disconnect", AuthMiddleware(AdminOnly(http.HandlerFunc(s.handler.DisconnectClient))))

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
	frontendFS, err := fs.Sub(s.webFS, "web/dist/client")
	if err != nil {
		log.Printf("Warning: failed to load frontend: %v", err)
	} else {
		// Serve static files
		fileServer := http.FileServer(http.FS(frontendFS))
		mux.Handle("/", spaHandler(frontendFS, fileServer))
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
