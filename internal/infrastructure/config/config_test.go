package config

import "testing"

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	t.Setenv("PROPULSE_HTTP_ADDR", "")
	t.Setenv("PROPULSE_DATABASE_URL", "")
	t.Setenv("PROPULSE_REDIS_ADDR", "")
	t.Setenv("PROPULSE_ACCESS_TOKEN", "")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "")
	t.Setenv("PROPULSE_LOG_LEVEL", "")
	t.Setenv("PROPULSE_LOG_PRETTY", "")
	t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL must have a local postgres default")
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("RedisAddr = %q, want 127.0.0.1:6379", cfg.RedisAddr)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty default", cfg.AccessToken)
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want info", cfg.Log.Level)
	}
	if cfg.Log.Pretty {
		t.Fatal("Log.Pretty must default to false")
	}
	if cfg.SchedulerInterval.String() != "1h0m0s" {
		t.Fatalf("SchedulerInterval = %s, want 1h0m0s", cfg.SchedulerInterval)
	}
}

func TestLoadReadsAccessToken(t *testing.T) {
	t.Setenv("PROPULSE_ACCESS_TOKEN", "secret-token")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "secret-token" {
		t.Fatalf("AccessToken = %q, want secret-token", cfg.AccessToken)
	}
}

func TestLoadDoesNotAcceptLegacyAdminToken(t *testing.T) {
	t.Setenv("PROPULSE_ACCESS_TOKEN", "")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty", cfg.AccessToken)
	}
}

func TestLoadParsesSchedulerInterval(t *testing.T) {
	t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "10s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SchedulerInterval.String() != "10s" {
		t.Fatalf("SchedulerInterval = %s, want 10s", cfg.SchedulerInterval)
	}
}

func TestLoadRejectsInvalidSchedulerInterval(t *testing.T) {
	t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "sometimes")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid interval error")
	}
}
