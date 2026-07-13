package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

func TestAdminDataSourcesCreateAndList(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	app := &stubDataSourceApplication{source: appcollection.DataSource{ID: "source-1", Name: "链家", SourceType: "manual_json", City: "杭州", CreatedAt: now, UpdatedAt: now}}
	handler := NewAdminDataSources(app)
	engine := gin.New()
	engine.POST("/data-sources", handler.Create)
	engine.GET("/data-sources", handler.List)

	create := httptest.NewRecorder()
	engine.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/data-sources", bytes.NewBufferString(`{"name":"链家","sourceType":"manual_json","city":"杭州"}`)))
	if create.Code != http.StatusCreated || app.command.SourceType != "manual_json" {
		t.Fatalf("create status/command = %d / %#v", create.Code, app.command)
	}

	list := httptest.NewRecorder()
	engine.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/data-sources", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"items"`)) {
		t.Fatalf("list status/body = %d / %s", list.Code, list.Body.String())
	}
}

type stubDataSourceApplication struct {
	source  appcollection.DataSource
	command appcollection.CreateDataSourceCommand
}

func (s *stubDataSourceApplication) CreateDataSource(_ context.Context, command appcollection.CreateDataSourceCommand) (appcollection.DataSource, error) {
	s.command = command
	return s.source, nil
}
func (s *stubDataSourceApplication) ListDataSources(context.Context, appcollection.ListDataSourcesQuery) ([]appcollection.DataSource, error) {
	return []appcollection.DataSource{s.source}, nil
}
