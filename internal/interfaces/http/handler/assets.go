package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	appasset "github.com/sine-io/propulse/internal/application/asset"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
)

type AssetApplication interface {
	CreateAsset(context.Context, appasset.CreateAssetCommand) (domainasset.Asset, error)
	UpdateAsset(context.Context, appasset.UpdateAssetCommand) (domainasset.Asset, error)
	DeleteAsset(context.Context, appasset.DeleteAssetCommand) error
	GetAsset(context.Context, appasset.GetAssetQuery) (domainasset.Asset, error)
	ListAssets(context.Context, appasset.ListAssetsQuery) (appasset.Page, error)
}

type Assets struct {
	app    AssetApplication
	userID string
}

func NewAssets(app AssetApplication, userID string) Assets {
	return Assets{app: app, userID: userID}
}

type propertySelectionRequest struct {
	Mode                   *appasset.PropertySelectionMode `json:"mode"`
	RoomID                 string                          `json:"roomId,omitempty"`
	Layout                 string                          `json:"layout,omitempty"`
	AreaSQM                *float64                        `json:"areaSqm,omitempty"`
	FloorBand              string                          `json:"floorBand,omitempty"`
	FloorDescription       string                          `json:"floorDescription,omitempty"`
	Orientation            string                          `json:"orientation,omitempty"`
	CurrentListingPriceWan *float64                        `json:"currentListingPriceWan,omitempty"`
}

type createAssetRequest struct {
	Name                     string                    `json:"name,omitempty"`
	NeighborhoodID           *string                   `json:"neighborhoodId"`
	PropertySelection        *propertySelectionRequest `json:"propertySelection"`
	OriginalPurchasePriceWan *float64                  `json:"originalPurchasePriceWan"`
	PurchasedOn              *string                   `json:"purchasedOn"`
	CurrentLoanBalanceWan    *float64                  `json:"currentLoanBalanceWan"`
}

type updateAssetRequest struct {
	Name                     *string                   `json:"name,omitempty"`
	PropertySelection        *propertySelectionRequest `json:"propertySelection,omitempty"`
	OriginalPurchasePriceWan *float64                  `json:"originalPurchasePriceWan,omitempty"`
	PurchasedOn              *string                   `json:"purchasedOn,omitempty"`
	CurrentLoanBalanceWan    *float64                  `json:"currentLoanBalanceWan,omitempty"`
}

type assetPropertyResponse struct {
	NeighborhoodID         string   `json:"neighborhoodId"`
	NeighborhoodName       string   `json:"neighborhoodName"`
	City                   string   `json:"city"`
	District               string   `json:"district"`
	Layout                 string   `json:"layout"`
	AreaSQM                float64  `json:"areaSqm"`
	FloorBand              string   `json:"floorBand"`
	FloorDescription       string   `json:"floorDescription"`
	Orientation            string   `json:"orientation"`
	CurrentListingPriceWan *float64 `json:"currentListingPriceWan"`
}

type assetListingSourceResponse struct {
	SourceListingID string `json:"sourceListingId"`
	DataSourceID    string `json:"dataSourceId"`
	DataSourceName  string `json:"dataSourceName"`
	DataSourceType  string `json:"dataSourceType"`
	SourceRef       string `json:"sourceRef"`
	CollectionRunID string `json:"collectionRunId"`
	SnapshotID      string `json:"snapshotId"`
	CollectedAt     string `json:"collectedAt"`
	ListedAt        string `json:"listedAt"`
	QualityStatus   string `json:"qualityStatus"`
}

type assetResponse struct {
	ID                       string                      `json:"id"`
	Name                     string                      `json:"name"`
	Property                 assetPropertyResponse       `json:"property"`
	OriginalPurchasePriceWan float64                     `json:"originalPurchasePriceWan"`
	PurchasedOn              string                      `json:"purchasedOn"`
	CurrentLoanBalanceWan    float64                     `json:"currentLoanBalanceWan"`
	SourceKind               domainasset.SourceKind      `json:"sourceKind"`
	ListingSource            *assetListingSourceResponse `json:"listingSource"`
	CreatedAt                string                      `json:"createdAt"`
	UpdatedAt                string                      `json:"updatedAt"`
}

type assetsPageResponse struct {
	Items    []assetResponse `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

func (h Assets) Create(c *gin.Context) {
	var request createAssetRequest
	if err := decodeStrictJSON(c, &request); err != nil || request.NeighborhoodID == nil || request.PropertySelection == nil ||
		request.OriginalPurchasePriceWan == nil || request.PurchasedOn == nil || request.CurrentLoanBalanceWan == nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	selection, err := request.PropertySelection.applicationInput()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	asset, err := h.app.CreateAsset(c.Request.Context(), appasset.CreateAssetCommand{
		UserID: h.userID, Name: request.Name, NeighborhoodID: *request.NeighborhoodID, PropertySelection: selection,
		OriginalPurchasePriceWan: *request.OriginalPurchasePriceWan, PurchasedOn: *request.PurchasedOn,
		CurrentLoanBalanceWan: *request.CurrentLoanBalanceWan,
	})
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusCreated, newAssetResponse(asset))
}

func (h Assets) List(c *gin.Context) {
	page, pageSize, err := assetPagination(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_query", "pagination parameters are invalid")
		return
	}
	result, err := h.app.ListAssets(c.Request.Context(), appasset.ListAssetsQuery{UserID: h.userID, Page: page, PageSize: pageSize})
	if err != nil {
		writeAssetError(c, err)
		return
	}
	items := make([]assetResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, newAssetResponse(item))
	}
	c.JSON(http.StatusOK, assetsPageResponse{Items: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize})
}

func (h Assets) Get(c *gin.Context) {
	asset, err := h.app.GetAsset(c.Request.Context(), appasset.GetAssetQuery{UserID: h.userID, ID: c.Param("id")})
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusOK, newAssetResponse(asset))
}

func (h Assets) Update(c *gin.Context) {
	var request updateAssetRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	var selection *appasset.PropertySelectionInput
	if request.PropertySelection != nil {
		value, err := request.PropertySelection.applicationInput()
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
			return
		}
		selection = &value
	}
	asset, err := h.app.UpdateAsset(c.Request.Context(), appasset.UpdateAssetCommand{
		UserID: h.userID, ID: c.Param("id"), Name: request.Name, PropertySelection: selection,
		OriginalPurchasePriceWan: request.OriginalPurchasePriceWan, PurchasedOn: request.PurchasedOn,
		CurrentLoanBalanceWan: request.CurrentLoanBalanceWan,
	})
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusOK, newAssetResponse(asset))
}

func (h Assets) Delete(c *gin.Context) {
	if err := h.app.DeleteAsset(c.Request.Context(), appasset.DeleteAssetCommand{UserID: h.userID, ID: c.Param("id")}); err != nil {
		writeAssetError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (request propertySelectionRequest) applicationInput() (appasset.PropertySelectionInput, error) {
	if request.Mode == nil {
		return appasset.PropertySelectionInput{}, appasset.ErrInvalidCommand
	}
	input := appasset.PropertySelectionInput{
		Mode: *request.Mode, RoomID: request.RoomID, Layout: request.Layout, FloorBand: request.FloorBand,
		FloorDescription: request.FloorDescription, Orientation: request.Orientation,
		CurrentListingPriceWan: request.CurrentListingPriceWan,
	}
	if request.AreaSQM != nil {
		input.AreaSQM = *request.AreaSQM
	}
	if input.Mode == appasset.PropertySelectionManual && request.AreaSQM == nil {
		return appasset.PropertySelectionInput{}, appasset.ErrInvalidCommand
	}
	return input, nil
}

func newAssetResponse(asset domainasset.Asset) assetResponse {
	response := assetResponse{
		ID: asset.ID, Name: asset.Name,
		Property: assetPropertyResponse{
			NeighborhoodID: asset.Property.NeighborhoodID, NeighborhoodName: asset.Property.NeighborhoodName,
			City: asset.Property.City, District: asset.Property.District, Layout: asset.Property.Layout,
			AreaSQM: asset.Property.AreaSQM, FloorBand: asset.Property.FloorBand,
			FloorDescription: asset.Property.FloorDescription, Orientation: asset.Property.Orientation,
			CurrentListingPriceWan: asset.Property.CurrentListingPriceWan,
		},
		OriginalPurchasePriceWan: asset.OriginalPurchasePriceWan, PurchasedOn: asset.PurchasedOn.Format(time.DateOnly),
		CurrentLoanBalanceWan: asset.CurrentLoanBalanceWan, SourceKind: asset.SourceKind,
		CreatedAt: asset.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: asset.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if asset.ListingSource != nil {
		response.ListingSource = &assetListingSourceResponse{
			SourceListingID: asset.ListingSource.SourceListingID, DataSourceID: asset.ListingSource.DataSourceID,
			DataSourceName: asset.ListingSource.DataSourceName, DataSourceType: asset.ListingSource.DataSourceType,
			SourceRef: asset.ListingSource.SourceRef, CollectionRunID: asset.ListingSource.CollectionRunID,
			SnapshotID: asset.ListingSource.SnapshotID, CollectedAt: asset.ListingSource.CollectedAt.UTC().Format(time.RFC3339),
			ListedAt: asset.ListingSource.ListedAt.UTC().Format(time.RFC3339), QualityStatus: asset.ListingSource.QualityStatus,
		}
	}
	return response
}

func decodeStrictJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON document")
	}
	return nil
}

func assetPagination(c *gin.Context) (int, int, error) {
	page, pageSize := 1, 20
	var err error
	if raw := c.Query("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, err
		}
	}
	if raw := c.Query("pageSize"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, err
		}
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return 0, 0, errors.New("invalid pagination")
	}
	return page, pageSize, nil
}

func writeAssetError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appasset.ErrInvalidCommand), errors.Is(err, domainasset.ErrInvalidAsset):
		writeError(c, http.StatusBadRequest, "invalid_request", "asset data is invalid")
	case errors.Is(err, appasset.ErrAssetNotFound):
		writeError(c, http.StatusNotFound, "not_found", "asset was not found")
	case errors.Is(err, appasset.ErrNeighborhoodNotFound), errors.Is(err, appasset.ErrListingNotFound):
		writeError(c, http.StatusNotFound, "source_not_found", "selected neighborhood or listing was not found")
	case errors.Is(err, appasset.ErrListingUnavailable):
		writeError(c, http.StatusConflict, "listing_unavailable", "selected listing is no longer active")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "asset operation failed")
	}
}
