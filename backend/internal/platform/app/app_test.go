package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	domaincapacity "github.com/propulse/propulse/backend/internal/domain/capacity"
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

func TestRunMigrateUpRunsMigrations(t *testing.T) {
	originalRunMigrations := runMigrations
	defer func() { runMigrations = originalRunMigrations }()

	called := false
	runMigrations = func(_ context.Context, databaseURL string, direction string) error {
		called = true
		if databaseURL != "postgres://test" {
			t.Fatalf("databaseURL = %q, want postgres://test", databaseURL)
		}
		if direction != "up" {
			t.Fatalf("direction = %q, want up", direction)
		}
		return nil
	}

	err := Run(context.Background(), "migrate up", config.Config{
		DatabaseURL: "postgres://test",
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(migrate up) error = %v", err)
	}
	if !called {
		t.Fatal("expected migrate runner to be called")
	}
}

func TestRunStartsAPIModeWithInjectedCapacityApplication(t *testing.T) {
	originalOpenCapacityApplication := openCapacityApplication
	originalListenAndServe := listenAndServe
	defer func() {
		openCapacityApplication = originalOpenCapacityApplication
		listenAndServe = originalListenAndServe
	}()

	service := &stubAppCapacityApplication{
		createRecord: appcapacity.CalculationRecord{
			ID: "calc_db",
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel: domaincapacity.PressureStrained,
				Strategy:      "先卖后买或同步推进",
			},
		},
	}

	opened := false
	openCapacityApplication = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (CapacityApplication, io.Closer, error) {
		opened = true
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return service, noopCloser{}, nil
	}

	listenAndServe = func(server *http.Server) error {
		body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
		req, err := http.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		rec := newInMemoryHTTPResponseWriter()

		server.Handler.ServeHTTP(rec, req)

		if rec.statusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rec.statusCode)
		}
		if !service.createCalled {
			t.Fatal("expected injected capacity application to handle request")
		}

		return http.ErrServerClosed
	}

	err := Run(context.Background(), "api", config.Config{
		HTTPAddr:    "127.0.0.1:0",
		DatabaseURL: "postgres://test",
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
	if !opened {
		t.Fatal("expected postgres capacity application to be opened")
	}
}

func testRunStartsHTTPServer(t *testing.T, mode string) {
	t.Helper()

	originalOpenCapacityApplication := openCapacityApplication
	defer func() { openCapacityApplication = originalOpenCapacityApplication }()
	openCapacityApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CapacityApplication, io.Closer, error) {
		return &stubAppCapacityApplication{}, noopCloser{}, nil
	}

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

type stubAppCapacityApplication struct {
	createCalled bool
	createRecord appcapacity.CalculationRecord
}

func (s *stubAppCapacityApplication) CreateCalculation(_ context.Context, _ appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error) {
	s.createCalled = true
	return s.createRecord, nil
}

func (s *stubAppCapacityApplication) GetCalculation(_ context.Context, _ appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error) {
	return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

type inMemoryHTTPResponseWriter struct {
	header     http.Header
	statusCode int
}

func newInMemoryHTTPResponseWriter() *inMemoryHTTPResponseWriter {
	return &inMemoryHTTPResponseWriter{
		header: http.Header{},
	}
}

func (w *inMemoryHTTPResponseWriter) Header() http.Header {
	return w.header
}

func (w *inMemoryHTTPResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return len(data), nil
}

func (w *inMemoryHTTPResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
