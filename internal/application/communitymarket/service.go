package communitymarket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() string
}

func NewService(repo Repository, now func() time.Time, newID func() string) *Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Service{repo: repo, now: now, newID: newID}
}

func (s *Service) ImportSnapshot(ctx context.Context, command ImportSnapshotCommand) (ImportSnapshotResult, error) {
	now := s.now().UTC()
	command = normalizeCommand(command)
	issues := validateCommand(command, now)
	if len(issues) > 0 {
		return ImportSnapshotResult{}, &ValidationError{Issues: issues}
	}

	exists, err := s.repo.DataSourceExists(ctx, command.DataSourceID)
	if err != nil {
		return ImportSnapshotResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	if !exists {
		return ImportSnapshotResult{}, ErrDataSourceNotFound
	}
	exists, err = s.repo.NeighborhoodExists(ctx, command.NeighborhoodID)
	if err != nil {
		return ImportSnapshotResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	if !exists {
		return ImportSnapshotResult{}, ErrNeighborhoodNotFound
	}

	snapshot := Snapshot{
		ID:              s.newID(),
		DataSourceID:    command.DataSourceID,
		NeighborhoodID:  command.NeighborhoodID,
		SourceRef:       command.SourceRef,
		CollectedAt:     command.CollectedAt.UTC(),
		ContentChecksum: checksum(command),
		RawPayload:      append([]byte(nil), command.RawPayload...),
		RawContentType:  command.RawContentType,
		Data:            command.Data,
		CreatedAt:       now,
	}
	saved, err := s.repo.SaveSnapshot(ctx, snapshot)
	if err != nil {
		return ImportSnapshotResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	return ImportSnapshotResult{Snapshot: saved.Snapshot, IdempotentReplay: !saved.Created}, nil
}

func (s *Service) LatestSnapshot(ctx context.Context, query LatestSnapshotQuery) (Snapshot, error) {
	id := strings.TrimSpace(query.NeighborhoodID)
	if _, err := uuid.Parse(id); err != nil {
		return Snapshot{}, ErrNeighborhoodNotFound
	}
	return s.repo.LatestSnapshot(ctx, id)
}

func normalizeCommand(command ImportSnapshotCommand) ImportSnapshotCommand {
	command.DataSourceID = strings.TrimSpace(command.DataSourceID)
	command.NeighborhoodID = strings.TrimSpace(command.NeighborhoodID)
	command.SourceRef = strings.TrimSpace(command.SourceRef)
	command.RawContentType = strings.TrimSpace(command.RawContentType)
	command.CollectedAt = command.CollectedAt.UTC()
	command.Data = command.Data.Normalize()
	return command
}

func validateCommand(command ImportSnapshotCommand, now time.Time) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if _, err := uuid.Parse(command.DataSourceID); err != nil {
		issues = append(issues, ValidationIssue{Field: "dataSourceId", Code: "invalid_uuid", Message: "dataSourceId must be a UUID"})
	}
	if _, err := uuid.Parse(command.NeighborhoodID); err != nil {
		issues = append(issues, ValidationIssue{Field: "neighborhoodId", Code: "invalid_uuid", Message: "neighborhoodId must be a UUID"})
	}
	if command.SourceRef == "" {
		issues = append(issues, ValidationIssue{Field: "sourceRef", Code: "required", Message: "sourceRef is required"})
	} else if utf8.RuneCountInString(command.SourceRef) > 256 {
		issues = append(issues, ValidationIssue{Field: "sourceRef", Code: "too_long", Message: "sourceRef must be at most 256 characters"})
	}
	if command.CollectedAt.IsZero() {
		issues = append(issues, ValidationIssue{Field: "collectedAt", Code: "required", Message: "collectedAt is required"})
	} else if command.CollectedAt.After(now.Add(5 * time.Minute)) {
		issues = append(issues, ValidationIssue{Field: "collectedAt", Code: "future", Message: "collectedAt must not be more than five minutes in the future"})
	}
	if len(command.RawPayload) == 0 {
		issues = append(issues, ValidationIssue{Field: "file", Code: "required", Message: "CSV payload is required"})
	} else if len(command.RawPayload) > MaxRawPayloadBytes {
		issues = append(issues, ValidationIssue{Field: "file", Code: "too_large", Message: "CSV payload must be at most 2 MiB"})
	}
	if command.RawContentType == "" {
		issues = append(issues, ValidationIssue{Field: "rawContentType", Code: "required", Message: "rawContentType is required"})
	} else if len(command.RawContentType) > 255 {
		issues = append(issues, ValidationIssue{Field: "rawContentType", Code: "too_long", Message: "rawContentType must be at most 255 bytes"})
	}
	for _, violation := range command.Data.Validate(command.CollectedAt) {
		row := 2
		issues = append(issues, ValidationIssue{Row: &row, Field: violation.Field, Code: violation.Code, Message: violation.Message})
	}
	return issues
}

func checksum(command ImportSnapshotCommand) string {
	hash := sha256.New()
	for _, value := range []string{
		command.DataSourceID,
		command.NeighborhoodID,
		command.SourceRef,
		command.CollectedAt.UTC().Format(time.RFC3339Nano),
		command.RawContentType,
	} {
		_, _ = fmt.Fprintf(hash, "%d:%s\n", len(value), value)
	}
	_, _ = hash.Write(command.RawPayload)
	return hex.EncodeToString(hash.Sum(nil))
}
