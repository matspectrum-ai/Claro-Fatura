package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
)

type Server struct {
	invoices *invoice.Service
	logger   *slog.Logger
}

func New(invoices *invoice.Service, logger *slog.Logger) http.Handler {
	s := &Server{invoices: invoices, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/faturas", s.queryInvoices)
	return securityHeaders(accessLog(logger, mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) queryInvoices(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("telefone"))
	result, err := s.invoices.QueryByPhone(r.Context(), phone)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "telefone inválido" { status = http.StatusBadRequest }
		writeJSON(w, status, map[string]string{"erro": publicError(status)})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func publicError(status int) string {
	if status == http.StatusBadRequest { return "Telefone inválido" }
	return "Não foi possível consultar no momento."
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}
