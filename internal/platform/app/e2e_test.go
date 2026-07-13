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
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("close GET %s response: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
	}

	unauthorized, err := http.Get(base + "/api/v1/watchlist")
	if err != nil {
		t.Fatalf("unauthenticated watchlist request failed: %v", err)
	}
	if err := unauthorized.Body.Close(); err != nil {
		t.Fatalf("close unauthenticated response: %v", err)
	}
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
	if err := authorized.Body.Close(); err != nil {
		t.Fatalf("close authenticated response: %v", err)
	}
	if authorized.StatusCode != http.StatusOK {
		t.Fatalf("authorized watchlist status = %d, want 200", authorized.StatusCode)
	}
}
