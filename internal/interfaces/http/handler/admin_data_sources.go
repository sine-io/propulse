package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

type DataSourceApplication interface {
	CreateDataSource(context.Context, appcollection.CreateDataSourceCommand) (appcollection.DataSource, error)
	ListDataSources(context.Context, appcollection.ListDataSourcesQuery) ([]appcollection.DataSource, error)
}

type AdminDataSources struct {
	app DataSourceApplication
}

func NewAdminDataSources(app DataSourceApplication) AdminDataSources {
	return AdminDataSources{app: app}
}

type createDataSourceRequest struct {
	Name       string `json:"name"`
	SourceType string `json:"sourceType"`
	City       string `json:"city"`
	Notes      string `json:"notes"`
}

type dataSourceResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"sourceType"`
	City       string `json:"city"`
	Notes      string `json:"notes"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

func (h AdminDataSources) Create(c *gin.Context) {
	var request createDataSourceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	source, err := h.app.CreateDataSource(c.Request.Context(), appcollection.CreateDataSourceCommand{
		Name: request.Name, SourceType: request.SourceType, City: request.City, Notes: request.Notes,
	})
	if err != nil {
		var validationErr *appcollection.ValidationError
		if errors.As(err, &validationErr) {
			writeValidationError(c, validationErr.Issues)
			return
		}
		writeError(c, http.StatusInternalServerError, "data_source_failed", "data source operation failed")
		return
	}
	c.JSON(http.StatusCreated, newDataSourceResponse(source))
}

func (h AdminDataSources) List(c *gin.Context) {
	sources, err := h.app.ListDataSources(c.Request.Context(), appcollection.ListDataSourcesQuery{})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "data_source_failed", "data source operation failed")
		return
	}
	items := make([]dataSourceResponse, 0, len(sources))
	for _, source := range sources {
		items = append(items, newDataSourceResponse(source))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func newDataSourceResponse(source appcollection.DataSource) dataSourceResponse {
	return dataSourceResponse{
		ID: source.ID, Name: source.Name, SourceType: source.SourceType, City: source.City, Notes: source.Notes,
		CreatedAt: source.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: source.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
