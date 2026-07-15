package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appreview "github.com/sine-io/propulse/internal/application/review"
)

const reviewDateLayout = "2006-01-02"

type ReviewApplication interface {
	CreateNote(ctx context.Context, command appreview.CreateNoteCommand) (appreview.Note, error)
	UpdateNote(ctx context.Context, command appreview.UpdateNoteCommand) (appreview.Note, error)
	GetNote(ctx context.Context, query appreview.GetNoteQuery) (appreview.Note, error)
	ListNotes(ctx context.Context, query appreview.ListNotesQuery) (appreview.NotesPage, error)
}

type Review struct {
	app ReviewApplication
}

func NewReview(app ReviewApplication) Review {
	return Review{app: app}
}

type createReviewNoteRequest struct {
	NeighborhoodID *string        `json:"neighborhoodId"`
	Kind           appreview.Kind `json:"kind"`
	WeekStartDate  *string        `json:"weekStartDate"`
	Content        string         `json:"content"`
}

type updateReviewNoteRequest struct {
	Content string `json:"content"`
}

type reviewNoteResponse struct {
	ID             string         `json:"id"`
	NeighborhoodID *string        `json:"neighborhoodId"`
	Kind           appreview.Kind `json:"kind"`
	WeekStartDate  *string        `json:"weekStartDate"`
	Content        string         `json:"content"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
}

type reviewNotesPageResponse struct {
	Items    []reviewNoteResponse `json:"items"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}

func (h Review) Create(c *gin.Context) {
	var request createReviewNoteRequest
	if err := decodeReviewJSON(c, &request); err != nil {
		writeReviewInvalidRequest(c)
		return
	}
	weekStartDate, err := parseReviewDate(request.WeekStartDate)
	if err != nil {
		writeReviewInvalidRequest(c)
		return
	}

	note, err := h.app.CreateNote(c.Request.Context(), appreview.CreateNoteCommand{
		NeighborhoodID: request.NeighborhoodID,
		Kind:           request.Kind,
		WeekStartDate:  weekStartDate,
		Content:        request.Content,
	})
	if err != nil {
		writeReviewError(c, err)
		return
	}
	c.JSON(http.StatusCreated, newReviewNoteResponse(note))
}

func (h Review) Get(c *gin.Context) {
	note, err := h.app.GetNote(c.Request.Context(), appreview.GetNoteQuery{ID: c.Param("id")})
	if err != nil {
		writeReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, newReviewNoteResponse(note))
}

func (h Review) Update(c *gin.Context) {
	var request updateReviewNoteRequest
	if err := decodeReviewJSON(c, &request); err != nil {
		writeReviewInvalidRequest(c)
		return
	}
	note, err := h.app.UpdateNote(c.Request.Context(), appreview.UpdateNoteCommand{
		ID:      c.Param("id"),
		Content: request.Content,
	})
	if err != nil {
		writeReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, newReviewNoteResponse(note))
}

func (h Review) List(c *gin.Context) {
	page, err := parseReviewPageParameter(c, "page")
	if err != nil {
		writeReviewInvalidRequest(c)
		return
	}
	pageSize, err := parseReviewPageParameter(c, "pageSize")
	if err != nil {
		writeReviewInvalidRequest(c)
		return
	}

	result, err := h.app.ListNotes(c.Request.Context(), appreview.ListNotesQuery{Page: page, PageSize: pageSize})
	if err != nil {
		writeReviewError(c, err)
		return
	}
	response := reviewNotesPageResponse{
		Items:    make([]reviewNoteResponse, 0, len(result.Items)),
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	}
	for _, note := range result.Items {
		response.Items = append(response.Items, newReviewNoteResponse(note))
	}
	c.JSON(http.StatusOK, response)
}

func decodeReviewJSON(c *gin.Context, target any) error {
	const maxBodyBytes = 128 << 10
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func parseReviewDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if *value == "" || strings.TrimSpace(*value) != *value {
		return nil, errors.New("invalid date")
	}
	parsed, err := time.Parse(reviewDateLayout, *value)
	if err != nil || parsed.Format(reviewDateLayout) != *value {
		return nil, errors.New("invalid date")
	}
	return &parsed, nil
}

func parseReviewPageParameter(c *gin.Context, name string) (int, error) {
	raw, present := c.GetQuery(name)
	if !present {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func newReviewNoteResponse(note appreview.Note) reviewNoteResponse {
	var weekStartDate *string
	if note.WeekStartDate != nil {
		formatted := note.WeekStartDate.Format(reviewDateLayout)
		weekStartDate = &formatted
	}
	return reviewNoteResponse{
		ID:             note.ID,
		NeighborhoodID: note.NeighborhoodID,
		Kind:           note.Kind,
		WeekStartDate:  weekStartDate,
		Content:        note.Content,
		CreatedAt:      note.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      note.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func writeReviewError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appreview.ErrInvalidNote),
		errors.Is(err, appreview.ErrInvalidNoteID),
		errors.Is(err, appreview.ErrInvalidNeighborhoodID),
		errors.Is(err, appreview.ErrInvalidPagination):
		writeReviewInvalidRequest(c)
	case errors.Is(err, appreview.ErrNeighborhoodNotFound):
		writeError(c, http.StatusNotFound, "not_found", "neighborhood not found")
	case errors.Is(err, appreview.ErrNoteNotFound):
		writeError(c, http.StatusNotFound, "not_found", "review note not found")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeReviewInvalidRequest(c *gin.Context) {
	writeError(c, http.StatusBadRequest, "invalid_request", "request is invalid")
}
