package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/propulse/propulse/backend/web"
	"github.com/rs/zerolog"
)

func TestHealthAndReadyRoutes(t *testing.T) {
	engine := New(Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: web.Embedded(),
	})

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
	}
}

func TestAPI404DoesNotReturnFrontend(t *testing.T) {
	engine := New(Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: web.Embedded(),
	})

	for _, path := range []string{"/api/v1/missing", "/admin/api/missing"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("%s content-type = %q, want application/json", path, got)
		}
		if strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("%s unexpectedly returned frontend html", path)
		}
	}
}

func TestFrontendRoutesServeEmbeddedHTML(t *testing.T) {
	engine := New(Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: web.Embedded(),
	})

	for _, path := range []string{"/", "/calculator", "/watchlist", "/action-window", "/neighborhoods", "/methods", "/templates"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
			t.Fatalf("%s content-type = %q, want text/html", path, got)
		}
		if !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("%s did not return embedded frontend html", path)
		}
	}
}

func TestRequestIDMiddlewareEchoesInboundHeaderAndLogsRequest(t *testing.T) {
	var logBuf bytes.Buffer
	engine := New(Dependencies{
		Log:      zerolog.New(&logBuf),
		StaticFS: web.Embedded(),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req-123")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "req-123" {
		t.Fatalf("response request id = %q, want req-123", got)
	}

	var entry map[string]any
	if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal(log) error = %v; raw=%q", err, logBuf.String())
	}
	if got := entry["request_id"]; got != "req-123" {
		t.Fatalf("logged request_id = %v, want req-123", got)
	}
	if got := entry["method"]; got != http.MethodGet {
		t.Fatalf("logged method = %v, want GET", got)
	}
	if got := entry["path"]; got != "/healthz" {
		t.Fatalf("logged path = %v, want /healthz", got)
	}
	if got := entry["status"]; got != float64(http.StatusOK) {
		t.Fatalf("logged status = %v, want 200", got)
	}
	if _, ok := entry["latency_ms"]; !ok {
		t.Fatalf("latency_ms missing from log entry: %v", entry)
	}
}

func TestRequestIDMiddlewareGeneratesHeaderWhenMissing(t *testing.T) {
	engine := New(Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: web.Embedded(),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatal("response missing generated X-Request-Id header")
	}
}

func TestRouterStopsWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := New(Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: web.Embedded(),
	})

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
