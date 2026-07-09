package app

import (
	"net/http"
	"os"
	"testing"
)

func TestE2ESmoke(t *testing.T) {
	base := os.Getenv("PROPULSE_E2E_BASE_URL")
	if base == "" {
		t.Skip("PROPULSE_E2E_BASE_URL is not set")
	}

	for _, path := range []string{"/healthz", "/", "/api/v1/watchlist"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			t.Fatalf("GET %s status = %d", path, resp.StatusCode)
		}
	}
}
