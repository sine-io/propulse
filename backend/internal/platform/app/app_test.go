package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/propulse/propulse/backend/internal/infrastructure/config"
	"github.com/rs/zerolog"
)

func TestNormalizeModeAcceptsDocumentedModes(t *testing.T) {
	for _, args := range [][]string{
		{"serve"},
		{"api"},
		{"worker"},
		{"scheduler"},
		{"migrate", "up"},
		{"migrate", "down"},
	} {
		mode, err := NormalizeMode(args)
		if err != nil {
			t.Fatalf("NormalizeMode(%v) error = %v", args, err)
		}
		if mode == "" {
			t.Fatalf("NormalizeMode(%v) returned empty mode", args)
		}
	}
}

func TestRunStartsHTTPServerForAPIMode(t *testing.T) {
	testRunStartsHTTPServer(t, "api")
}

func TestRunStartsHTTPServerForServeMode(t *testing.T) {
	testRunStartsHTTPServer(t, "serve")
}

func testRunStartsHTTPServer(t *testing.T, mode string) {
	t.Helper()

	addr := freeLocalAddr(t)
	cfg := config.Config{
		HTTPAddr: addr,
		Log: config.LogConfig{
			Level: "info",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, mode, cfg, zerolog.New(io.Discard))
	}()

	waitForHTTP(t, "http://"+addr+"/healthz")
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run(%q) error = %v", mode, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run(%q) did not stop after cancel", mode)
	}
}

func freeLocalAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	defer ln.Close()

	return ln.Addr().String()
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("server at %s did not become healthy before timeout", url)
}

func TestRunRejectsUnknownMode(t *testing.T) {
	err := Run(context.Background(), "unknown", config.Config{}, zerolog.New(io.Discard))
	if err == nil {
		t.Fatal("Run returned nil for unknown mode")
	}
	if got, want := err.Error(), Usage; got != want {
		t.Fatalf("Run error = %q, want %q", got, want)
	}
}

func TestUsageStringMatchesDocumentedCLI(t *testing.T) {
	want := "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"
	if Usage != want {
		t.Fatalf("Usage = %q, want %q", Usage, want)
	}
}

func ExampleNormalizeMode() {
	mode, _ := NormalizeMode([]string{"api"})
	fmt.Println(mode)
	// Output: api
}
