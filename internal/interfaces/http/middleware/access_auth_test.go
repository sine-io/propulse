package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessAuth(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		header     string
		wantStatus int
	}{
		{name: "missing configuration", configured: "", header: "Bearer secret", wantStatus: http.StatusUnauthorized},
		{name: "whitespace-only configuration", configured: " \t ", header: "Bearer secret", wantStatus: http.StatusUnauthorized},
		{name: "missing header", configured: "secret", header: "", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", configured: "secret", header: "Basic secret", wantStatus: http.StatusUnauthorized},
		{name: "empty bearer", configured: "secret", header: "Bearer ", wantStatus: http.StatusUnauthorized},
		{name: "wrong token", configured: "secret", header: "Bearer wrong", wantStatus: http.StatusUnauthorized},
		{name: "valid token", configured: "secret", header: "Bearer secret", wantStatus: http.StatusNoContent},
		{name: "padded configuration", configured: "  secret\t", header: "Bearer   secret  ", wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			called := false
			engine := gin.New()
			engine.Use(AccessAuth(tt.configured))
			engine.GET("/", func(c *gin.Context) {
				called = true
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusNoContent {
				if !called {
					t.Fatal("authenticated request did not reach handler")
				}
				return
			}

			if called {
				t.Fatal("unauthenticated request reached handler")
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
			}
			var response struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("json.Unmarshal() error = %v; body=%s", err, rec.Body.String())
			}
			if response.Error.Code != "access_required" {
				t.Fatalf("error code = %q, want access_required", response.Error.Code)
			}
		})
	}
}
