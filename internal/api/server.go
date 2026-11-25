package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/kavix/k8s-advisor/internal/advisor"
	"github.com/kavix/k8s-advisor/internal/models"
	"github.com/kavix/k8s-advisor/internal/storage"
)

// Server exposes HTTP endpoints for health checks, advisory listing, and on-demand event analysis.
type Server struct {
	store  storage.AdvisorStore
	engine *advisor.Engine
	mux    *http.ServeMux
}

// NewServer constructs an HTTP Server with registered routes.
func NewServer(store storage.AdvisorStore, engine *advisor.Engine) *Server {
	s := &Server{
		store:  store,
		engine: engine,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /api/v1/advisories", s.handleListAdvisories)
	s.mux.HandleFunc("POST /api/v1/analyze", s.handleAnalyzeEvent)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "no_store"})
		return
	}
	if err := s.store.HealthCheck(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_unreachable", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleListAdvisories(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	advisories, err := s.store.ListAdvisories(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"advisories": advisories})
}

func (s *Server) handleAnalyzeEvent(w http.ResponseWriter, r *http.Request) {
	var event models.K8sEventRecord
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	adv, err := s.engine.AnalyzeEvent(r.Context(), event)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, adv)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
