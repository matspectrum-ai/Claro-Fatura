package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Addr                   string
	SupabaseURL            string
	SupabaseServiceRoleKey string
	SupabasePublishableKey string
	SiteURL                string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                   envOr("ADDR", ":8080"),
		SupabaseURL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabasePublishableKey: firstNonEmpty(os.Getenv("SUPABASE_PUBLISHABLE_KEY"), os.Getenv("SUPABASE_ANON_KEY")),
		SiteURL:                strings.TrimRight(envOr("SITE_URL", "https://clarofatura.app"), "/"),
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
