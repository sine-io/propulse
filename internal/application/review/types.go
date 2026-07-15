// Package review coordinates review-note commands and queries.
package review

import (
	"context"
	"errors"
	"strings"
	"time"

	domainreview "github.com/sine-io/propulse/internal/domain/review"
)

type Kind = domainreview.Kind
type Note = domainreview.Note

const (
	KindReview      = domainreview.KindReview
	KindViewingNote = domainreview.KindViewingNote
	MaxContentRunes = domainreview.MaxContentRunes
)

var ErrInvalidNote = domainreview.ErrInvalidNote
var ErrInvalidNoteID = errors.New("invalid review note id")
var ErrInvalidNeighborhoodID = errors.New("invalid neighborhood id")
var ErrInvalidPagination = errors.New("invalid review note pagination")
var ErrNeighborhoodNotFound = errors.New("neighborhood not found")
var ErrNoteNotFound = errors.New("review note not found")

type CreateNoteInput struct {
	UserID         string
	NeighborhoodID *string
	Kind           Kind
	WeekStartDate  *time.Time
	Content        string
}

func (input CreateNoteInput) Validate() error {
	if strings.TrimSpace(input.UserID) == "" || !domainreview.IsValidKind(input.Kind) {
		return ErrInvalidNote
	}
	if _, err := domainreview.NormalizeContent(input.Content); err != nil {
		return err
	}
	if input.NeighborhoodID != nil {
		_, err := normalizeRequiredUUID(*input.NeighborhoodID, ErrInvalidNeighborhoodID)
		return err
	}
	return nil
}

type ListNotesInput struct {
	UserID string
	Limit  int
	Offset int
}

type ListNotesResult struct {
	Items []Note
	Total int
}

type Repository interface {
	CreateNote(ctx context.Context, input CreateNoteInput) (Note, error)
	NeighborhoodExists(ctx context.Context, id string) (bool, error)
	FindNote(ctx context.Context, userID string, id string) (Note, error)
	UpdateNoteContent(ctx context.Context, userID string, id string, content string) (Note, error)
	ListNotes(ctx context.Context, input ListNotesInput) (ListNotesResult, error)
}
