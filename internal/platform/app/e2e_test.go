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
	token := os.Getenv("PROPULSE_E2E_ACCESS_TOKEN")
	if token == "" {
		t.Skip("PROPULSE_E2E_ACCESS_TOKEN is not set")
	}

	for _, path := range []string{"/healthz", "/readyz", "/"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
	}

	unauthorized, err := http.Get(base + "/api/v1/watchlist")
	if err != nil {
		t.Fatalf("unauthenticated watchlist request failed: %v", err)
	}
	unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated watchlist status = %d, want 401", unauthorized.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/watchlist", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	authorized, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authorized watchlist request failed: %v", err)
	}
	authorized.Body.Close()
	if authorized.StatusCode != http.StatusOK {
		t.Fatalf("authorized watchlist status = %d, want 200", authorized.StatusCode)
	}
}
