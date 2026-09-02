package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
	authsvc "github.com/matspectrum-ai/Claro-Fatura/internal/auth"
	"github.com/matspectrum-ai/Claro-Fatura/internal/config"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/cashinpay"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/generic"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/m2pay"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/nowbanks"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/propix"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway/staticpix"
	"github.com/matspectrum-ai/Claro-Fatura/internal/httpapi"
	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Claro-Fatura/internal/legacy"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
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
	fallback := generic.New()
	propixClient := propix.New(cfg.ProPixClientID, cfg.ProPixClientSecret, cfg.ProductName, gateway.DefaultCustomerEmail)
	registry := gateway.NewRegistry(
		fallback,
		cashinpay.New(cfg.CashinPaySecretKey, cfg.CashinPayWebhookSecret, cfg.ProductName, gateway.DefaultCustomerEmail),
		propixClient,
		m2pay.New(cfg.M2PayAPIKey, cfg.ProductName, gateway.DefaultCustomerEmail),
		nowbanks.New(cfg.NowBanksClientID, cfg.NowBanksClientSecret, cfg.NowBanksWebhookSecret),
		staticpix.New(cfg.PIXKey, cfg.PIXReceiver, cfg.PIXCity),
	)

	paymentRouter := payment.NewWithExpiration(store, registry, cfg.ProductName, cfg.PIXExpiration)
	generator := payment.NewGenerator(store, paymentRouter)
	confirmer := payment.NewConfirmer(store, registry)
	status := payment.NewStatusService(store, confirmer)
	webhooks := payment.NewWebhookService(confirmer)
	adminAuth := authsvc.New(cfg.SupabaseURL, cfg.SupabasePublishableKey, store)
	adminInvoices := admin.NewInvoiceService(store)
	adminImporter := admin.NewImporter(store)
	adminMetrics := admin.NewMetricsService(store)
	adminActivity := admin.NewActivityService(store)
	adminGateways := admin.NewGatewayAdminService(store, os.Getenv)
	legacyAPI := legacy.New(store, paymentRouter, propixClient, cfg.SiteURL)

	handler := httpapi.New(httpapi.Dependencies{
		Invoices:      invoice.New(store),
		PIX:           generator,
		Status:        status,
		Webhooks:      webhooks,
		Access:        store,
		Auth:          adminAuth,
		AdminInvoices: adminInvoices,
		AdminImporter: adminImporter,
		AdminMetrics:  adminMetrics,
		AdminActivity: adminActivity,
		AdminGateways: adminGateways,
		Legacy:        legacyAPI,
		SiteURL:       cfg.SiteURL,
	}, logger)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server started", "addr", cfg.Addr)
		serverErr <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-signalCtx.Done():
		logger.Info("shutdown started")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			_ = server.Close()
			os.Exit(1)
		}
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped during shutdown", "error", err)
			os.Exit(1)
		}
		logger.Info("shutdown complete")
	}
}
