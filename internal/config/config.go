package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type Config struct {
	Addr                   string
	SupabaseURL            string
	SupabaseServiceRoleKey string
	SupabasePublishableKey string
	SiteURL                string
	ProductName            string
	PIXExpiration          time.Duration

	CashinPaySecretKey     string
	CashinPayWebhookSecret string
	ProPixClientID         string
	ProPixClientSecret     string
	M2PayAPIKey            string
	NowBanksClientID       string
	NowBanksClientSecret   string
	NowBanksWebhookSecret  string
	PIXKey                 string
	PIXReceiver            string
	PIXCity                string
}

func Load() (Config, error) {
	minutes := envInt("PIX_EXPIRACAO_MINUTOS", 30)
	if minutes <= 0 {
		minutes = 30
	}
	cfg := Config{
		Addr:                   envOr("ADDR", ":8080"),
		SupabaseURL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabasePublishableKey: firstNonEmpty(os.Getenv("SUPABASE_PUBLISHABLE_KEY"), os.Getenv("SUPABASE_ANON_KEY")),
		SiteURL:                strings.TrimRight(envOr("SITE_URL", "https://clarofatura.app"), "/"),
		ProductName:            envOr("PRODUTO_NOME", gateway.DefaultProductName),
		PIXExpiration:          time.Duration(minutes) * time.Minute,
		CashinPaySecretKey:     os.Getenv("CASHINPAY_SECRET_KEY"),
		CashinPayWebhookSecret: os.Getenv("CASHINPAY_WEBHOOK_SECRET"),
		ProPixClientID:         os.Getenv("PROPIX_CLIENT_ID"),
		ProPixClientSecret:     os.Getenv("PROPIX_CLIENT_SECRET"),
		M2PayAPIKey:            os.Getenv("M2PAY_API_KEY"),
		NowBanksClientID:       os.Getenv("NOWBANKS_CLIENT_ID"),
		NowBanksClientSecret:   os.Getenv("NOWBANKS_CLIENT_SECRET"),
		NowBanksWebhookSecret:  os.Getenv("NOWBANKS_WEBHOOK_SECRET"),
		PIXKey:                 os.Getenv("PIX_CHAVE"),
		PIXReceiver:            firstNonEmpty(os.Getenv("PIX_RECEBEDOR"), os.Getenv("PIX_NOME"), "FATURA MOVEL"),
		PIXCity:                envOr("PIX_CIDADE", "SAO PAULO"),
	}

	var missing []string
	if cfg.SupabaseURL == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if cfg.SupabaseServiceRoleKey == "" {
		missing = append(missing, "SUPABASE_SERVICE_ROLE_KEY")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
