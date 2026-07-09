package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const manualJSONSourceType = "manual_json"

type Service struct {
	repo             Repository
	metricCalculator MetricCalculator
	now              func() time.Time
	newID            func() string
}

func NewService(repo Repository, now func() time.Time, newID func() string, metricCalculator ...MetricCalculator) *Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	var calculator MetricCalculator
	if len(metricCalculator) > 0 {
		calculator = metricCalculator[0]
	}
	return &Service{repo: repo, metricCalculator: calculator, now: now, newID: newID}
}

type ImportManualListingsCommand struct {
	SourceType     string                `json:"sourceType"`
	SourceRef      string                `json:"sourceRef"`
	NeighborhoodID string                `json:"neighborhoodId"`
	Records        []ManualListingRecord `json:"records"`
}

type ManualListingRecord struct {
	ListingPrice     float64  `json:"listingPrice"`
	TransactionPrice *float64 `json:"transactionPrice"`
	PriceCut         bool     `json:"priceCut"`
	DaysOnMarket     int      `json:"daysOnMarket"`
	Layout           string   `json:"layout"`
}

type ImportManualListingsResult struct {
	CollectionRunID       string
	ImportedSnapshotCount int
}

func (s *Service) ImportManualListings(ctx context.Context, command ImportManualListingsCommand) (ImportManualListingsResult, error) {
	command.SourceType = strings.TrimSpace(command.SourceType)
	command.SourceRef = strings.TrimSpace(command.SourceRef)
	command.NeighborhoodID = strings.TrimSpace(command.NeighborhoodID)
	if err := validateImport(command); err != nil {
		return ImportManualListingsResult{}, err
	}

	exists, err := s.repo.NeighborhoodExists(ctx, command.NeighborhoodID)
	if err != nil {
		return ImportManualListingsResult{}, fmt.Errorf("%w: %v", ErrImportFailed, err)
	}
	if !exists {
		return ImportManualListingsResult{}, ErrNeighborhoodNotFound
	}

	collectedAt := s.now().UTC()
	raw := RawCollectionRecord{
		ID:          s.newID(),
		SourceType:  command.SourceType,
		SourceRef:   command.SourceRef,
		CollectedAt: collectedAt,
	}
	raw.Payload, err = json.Marshal(command)
	if err != nil {
		return ImportManualListingsResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	snapshots := make([]ListingSnapshot, 0, len(command.Records))
	for _, record := range command.Records {
		snapshots = append(snapshots, ListingSnapshot{
			ID:               s.newID(),
			CollectionRunID:  raw.ID,
			NeighborhoodID:   command.NeighborhoodID,
			ListingPrice:     record.ListingPrice,
			TransactionPrice: record.TransactionPrice,
			PriceCut:         record.PriceCut,
			DaysOnMarket:     record.DaysOnMarket,
			Layout:           record.Layout,
			CapturedAt:       collectedAt,
		})
	}

	if err := s.repo.SaveImport(ctx, raw, snapshots); err != nil {
		return ImportManualListingsResult{}, fmt.Errorf("%w: %v", ErrImportFailed, err)
	}
	if s.metricCalculator != nil {
		if err := s.metricCalculator.CalculateNeighborhood(ctx, command.NeighborhoodID); err != nil {
			return ImportManualListingsResult{}, fmt.Errorf("%w: %v", ErrImportFailed, err)
		}
	}

	return ImportManualListingsResult{
		CollectionRunID:       raw.ID,
		ImportedSnapshotCount: len(snapshots),
	}, nil
}

func validateImport(command ImportManualListingsCommand) error {
	if command.SourceType != manualJSONSourceType {
		return ErrInvalidRequest
	}
	if command.SourceRef == "" {
		return ErrInvalidRequest
	}
	if command.NeighborhoodID == "" {
		return ErrInvalidRequest
	}
	if len(command.Records) < 1 || len(command.Records) > 500 {
		return ErrInvalidRequest
	}
	for _, record := range command.Records {
		if record.ListingPrice <= 0 {
			return ErrInvalidRequest
		}
		if record.TransactionPrice != nil && *record.TransactionPrice <= 0 {
			return ErrInvalidRequest
		}
		if record.DaysOnMarket < 0 {
			return ErrInvalidRequest
		}
	}
	return nil
}
