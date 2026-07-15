package review

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	domainreview "github.com/sine-io/propulse/internal/domain/review"
)

type Service struct {
	repo   Repository
	userID string
}

func NewService(repo Repository, userID string) *Service {
	return &Service{repo: repo, userID: strings.TrimSpace(userID)}
}

type CreateNoteCommand struct {
	NeighborhoodID *string
	Kind           Kind
	WeekStartDate  *time.Time
	Content        string
}

func (s *Service) CreateNote(ctx context.Context, command CreateNoteCommand) (Note, error) {
	content, err := domainreview.NormalizeContent(command.Content)
	if err != nil || !domainreview.IsValidKind(command.Kind) || s.userID == "" {
		return Note{}, ErrInvalidNote
	}

	neighborhoodID, err := normalizeOptionalUUID(command.NeighborhoodID)
	if err != nil {
		return Note{}, err
	}
	if neighborhoodID != nil {
		exists, err := s.repo.NeighborhoodExists(ctx, *neighborhoodID)
		if err != nil {
			return Note{}, err
		}
		if !exists {
			return Note{}, ErrNeighborhoodNotFound
		}
	}

	return s.repo.CreateNote(ctx, CreateNoteInput{
		UserID:         s.userID,
		NeighborhoodID: neighborhoodID,
		Kind:           command.Kind,
		WeekStartDate:  normalizeDate(command.WeekStartDate),
		Content:        content,
	})
}

type UpdateNoteCommand struct {
	ID      string
	Content string
}

func (s *Service) UpdateNote(ctx context.Context, command UpdateNoteCommand) (Note, error) {
	id, err := normalizeRequiredUUID(command.ID, ErrInvalidNoteID)
	if err != nil {
		return Note{}, err
	}
	content, err := domainreview.NormalizeContent(command.Content)
	if err != nil || s.userID == "" {
		return Note{}, ErrInvalidNote
	}
	return s.repo.UpdateNoteContent(ctx, s.userID, id, content)
}

func normalizeOptionalUUID(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized, err := normalizeRequiredUUID(*value, ErrInvalidNeighborhoodID)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeRequiredUUID(value string, invalidErr error) (string, error) {
	if strings.TrimSpace(value) != value || len(value) != len(uuid.Nil.String()) {
		return "", invalidErr
	}
	parsed, err := uuid.Parse(value)
	if err != nil || !strings.EqualFold(value, parsed.String()) {
		return "", invalidErr
	}
	return parsed.String(), nil
}

func normalizeDate(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	year, month, day := value.Date()
	normalized := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return &normalized
}
