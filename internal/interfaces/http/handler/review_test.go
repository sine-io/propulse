package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appreview "github.com/sine-io/propulse/internal/application/review"
)

func TestReviewCreateParsesRequestAndHidesUserID(t *testing.T) {
	neighborhoodID := "11111111-1111-4111-8111-111111111111"
	week := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 7, 15, 1, 2, 3, 456000000, time.UTC)
	app := &reviewApplicationStub{note: appreview.Note{
		ID:             "22222222-2222-4222-8222-222222222222",
		UserID:         "must-not-leak",
		NeighborhoodID: &neighborhoodID,
		Kind:           appreview.KindReview,
		WeekStartDate:  &week,
		Content:        "本周复盘",
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}}
	engine := newReviewHandlerTestEngine(app)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/review-notes", strings.NewReader(`{"kind":"review","content":"本周复盘","neighborhoodId":"11111111-1111-4111-8111-111111111111","weekStartDate":"2026-07-13"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if app.createCommand.Kind != appreview.KindReview || app.createCommand.Content != "本周复盘" || app.createCommand.WeekStartDate == nil || !app.createCommand.WeekStartDate.Equal(week) {
		t.Fatalf("create command = %#v", app.createCommand)
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, present := response["userId"]; present {
		t.Fatal("response exposes userId")
	}
	if response["weekStartDate"] != "2026-07-13" || response["createdAt"] != "2026-07-15T01:02:03.456Z" {
		t.Fatalf("response = %#v", response)
	}
}

func TestReviewGetUpdateAndListResponses(t *testing.T) {
	now := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	note := appreview.Note{ID: "33333333-3333-4333-8333-333333333333", Kind: appreview.KindViewingNote, Content: "note", CreatedAt: now, UpdatedAt: now}
	app := &reviewApplicationStub{note: note, page: appreview.NotesPage{Items: []appreview.Note{note}, Total: 21, Page: 2, PageSize: 10}}
	engine := newReviewHandlerTestEngine(app)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "get", method: http.MethodGet, path: "/api/v1/review-notes/33333333-3333-4333-8333-333333333333", wantStatus: http.StatusOK},
		{name: "update", method: http.MethodPatch, path: "/api/v1/review-notes/33333333-3333-4333-8333-333333333333", body: `{"content":"updated"}`, wantStatus: http.StatusOK},
		{name: "list", method: http.MethodGet, path: "/api/v1/review-notes?page=2&pageSize=10", wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.name == "list" {
				var response reviewNotesPageResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode list response: %v", err)
				}
				if response.Total != 21 || response.Page != 2 || response.PageSize != 10 || len(response.Items) != 1 || response.Items[0].NeighborhoodID != nil || response.Items[0].WeekStartDate != nil {
					t.Fatalf("list response = %#v", response)
				}
			}
		})
	}
	if app.getQuery.ID != note.ID || app.updateCommand.ID != note.ID || app.updateCommand.Content != "updated" {
		t.Fatalf("get query = %#v, update command = %#v", app.getQuery, app.updateCommand)
	}
	if app.listQuery.Page != 2 || app.listQuery.PageSize != 10 {
		t.Fatalf("list query = %#v", app.listQuery)
	}
}

func TestReviewRejectsMalformedOrUnsupportedRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "malformed json", method: http.MethodPost, path: "/api/v1/review-notes", body: `{"kind":`},
		{name: "unknown create field", method: http.MethodPost, path: "/api/v1/review-notes", body: `{"kind":"review","content":"x","userId":"other"}`},
		{name: "trailing json", method: http.MethodPost, path: "/api/v1/review-notes", body: `{"kind":"review","content":"x"} {}`},
		{name: "invalid date", method: http.MethodPost, path: "/api/v1/review-notes", body: `{"kind":"review","content":"x","weekStartDate":"2026-02-30"}`},
		{name: "update immutable field", method: http.MethodPatch, path: "/api/v1/review-notes/33333333-3333-4333-8333-333333333333", body: `{"content":"x","kind":"review"}`},
		{name: "invalid page", method: http.MethodGet, path: "/api/v1/review-notes?page=zero"},
		{name: "zero page size", method: http.MethodGet, path: "/api/v1/review-notes?pageSize=0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &reviewApplicationStub{}
			engine := newReviewHandlerTestEngine(app)
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assertReviewErrorResponse(t, rec, http.StatusBadRequest, "invalid_request")
			if app.calls != 0 {
				t.Fatalf("application calls = %d, want 0", app.calls)
			}
		})
	}
}

func TestReviewMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid request", err: appreview.ErrInvalidNote, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "invalid id", err: appreview.ErrInvalidNoteID, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "missing neighborhood", err: appreview.ErrNeighborhoodNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "missing or unowned note", err: appreview.ErrNoteNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "storage error", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newReviewHandlerTestEngine(&reviewApplicationStub{err: tt.err})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/review-notes/33333333-3333-4333-8333-333333333333", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assertReviewErrorResponse(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}

func newReviewHandlerTestEngine(app ReviewApplication) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	handler := NewReview(app)
	engine.POST("/api/v1/review-notes", handler.Create)
	engine.GET("/api/v1/review-notes", handler.List)
	engine.GET("/api/v1/review-notes/:id", handler.Get)
	engine.PATCH("/api/v1/review-notes/:id", handler.Update)
	return engine
}

func assertReviewErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, status, rec.Body.String())
	}
	var response errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Error.Code != code {
		t.Fatalf("error code = %q, want %q", response.Error.Code, code)
	}
}

type reviewApplicationStub struct {
	note          appreview.Note
	page          appreview.NotesPage
	err           error
	createCommand appreview.CreateNoteCommand
	updateCommand appreview.UpdateNoteCommand
	getQuery      appreview.GetNoteQuery
	listQuery     appreview.ListNotesQuery
	calls         int
}

func (s *reviewApplicationStub) CreateNote(_ context.Context, command appreview.CreateNoteCommand) (appreview.Note, error) {
	s.calls++
	s.createCommand = command
	return s.note, s.err
}

func (s *reviewApplicationStub) UpdateNote(_ context.Context, command appreview.UpdateNoteCommand) (appreview.Note, error) {
	s.calls++
	s.updateCommand = command
	return s.note, s.err
}

func (s *reviewApplicationStub) GetNote(_ context.Context, query appreview.GetNoteQuery) (appreview.Note, error) {
	s.calls++
	s.getQuery = query
	return s.note, s.err
}

func (s *reviewApplicationStub) ListNotes(_ context.Context, query appreview.ListNotesQuery) (appreview.NotesPage, error) {
	s.calls++
	s.listQuery = query
	return s.page, s.err
}
