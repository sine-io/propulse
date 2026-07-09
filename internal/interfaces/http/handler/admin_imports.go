package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

type CollectionApplication interface {
	ImportManualListings(ctx context.Context, command appcollection.ImportManualListingsCommand) (appcollection.ImportManualListingsResult, error)
}

type AdminImports struct {
	app CollectionApplication
}

func NewAdminImports(app CollectionApplication) AdminImports {
	return AdminImports{app: app}
}

type createImportRequest struct {
	SourceType     string                              `json:"sourceType"`
	SourceRef      string                              `json:"sourceRef"`
	NeighborhoodID string                              `json:"neighborhoodId"`
	Records        []appcollection.ManualListingRecord `json:"records"`
}

type createImportResponse struct {
	CollectionRunID       string `json:"collectionRunId"`
	ImportedSnapshotCount int    `json:"importedSnapshotCount"`
}

func (h AdminImports) CreateImport(c *gin.Context) {
	var request createImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	result, err := h.app.ImportManualListings(c.Request.Context(), appcollection.ImportManualListingsCommand{
		SourceType:     request.SourceType,
		SourceRef:      request.SourceRef,
		NeighborhoodID: request.NeighborhoodID,
		Records:        request.Records,
	})
	if err != nil {
		switch {
		case errors.Is(err, appcollection.ErrInvalidRequest):
			writeError(c, http.StatusBadRequest, "invalid_request", "request is invalid")
		case errors.Is(err, appcollection.ErrNeighborhoodNotFound):
			writeError(c, http.StatusNotFound, "neighborhood_not_found", "neighborhood not found")
		case errors.Is(err, appcollection.ErrImportFailed):
			writeError(c, http.StatusInternalServerError, "import_failed", "import failed")
		default:
			writeError(c, http.StatusInternalServerError, "import_failed", "import failed")
		}
		return
	}

	c.JSON(http.StatusCreated, createImportResponse{
		CollectionRunID:       result.CollectionRunID,
		ImportedSnapshotCount: result.ImportedSnapshotCount,
	})
}
