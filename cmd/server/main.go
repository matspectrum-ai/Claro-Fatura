package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/config"
	"github.com/matspectrum-ai/Claro-Fatura/internal/httpapi"
	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Claro-Fatura/internal/supabase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	store := supabase.New(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey)
	handler := httpapi.New(invoice.New(store), logger)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	logger.Info("server started", "addr", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
