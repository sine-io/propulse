package config

import "testing"

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	t.Setenv("PROPULSE_HTTP_ADDR", "")
	t.Setenv("PROPULSE_DATABASE_URL", "")
	t.Setenv("PROPULSE_REDIS_ADDR", "")
	t.Setenv("PROPULSE_LOG_LEVEL", "")
	t.Setenv("PROPULSE_LOG_PRETTY", "")
	t.Setenv("PROPULSE_SEED_DEMO_DATA", "")

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
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want info", cfg.Log.Level)
	}
	if cfg.Log.Pretty {
		t.Fatal("Log.Pretty must default to false")
	}
	if cfg.SeedDemoData {
		t.Fatal("SeedDemoData must default to false")
	}
}

func TestLoadEnablesDemoSeedData(t *testing.T) {
	t.Setenv("PROPULSE_SEED_DEMO_DATA", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.SeedDemoData {
		t.Fatal("SeedDemoData = false, want true")
	}
}
