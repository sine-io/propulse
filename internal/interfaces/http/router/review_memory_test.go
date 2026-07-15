package router

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	appreview "github.com/sine-io/propulse/internal/application/review"
)

type inMemoryReviewRepository struct {
	mu    sync.Mutex
	notes map[string]appreview.Note
}

func newInMemoryReviewRepository() *inMemoryReviewRepository {
	return &inMemoryReviewRepository{notes: make(map[string]appreview.Note)}
}

func (r *inMemoryReviewRepository) CreateNote(_ context.Context, input appreview.CreateNoteInput) (appreview.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	note := appreview.Note{
		ID:             uuid.NewString(),
		UserID:         input.UserID,
		NeighborhoodID: input.NeighborhoodID,
		Kind:           input.Kind,
		WeekStartDate:  input.WeekStartDate,
		Content:        input.Content,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.notes[note.ID] = note
	return note, nil
}

func (*inMemoryReviewRepository) NeighborhoodExists(context.Context, string) (bool, error) {
	return true, nil
}

func (r *inMemoryReviewRepository) FindNote(_ context.Context, userID string, id string) (appreview.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	note, ok := r.notes[id]
	if !ok || note.UserID != userID {
		return appreview.Note{}, appreview.ErrNoteNotFound
	}
	return note, nil
}

func (r *inMemoryReviewRepository) UpdateNoteContent(_ context.Context, userID string, id string, content string) (appreview.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	note, ok := r.notes[id]
	if !ok || note.UserID != userID {
		return appreview.Note{}, appreview.ErrNoteNotFound
	}
	note.Content = content
	note.UpdatedAt = time.Now().UTC()
	r.notes[id] = note
	return note, nil
}

func (r *inMemoryReviewRepository) ListNotes(_ context.Context, input appreview.ListNotesInput) (appreview.ListNotesResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]appreview.Note, 0, len(r.notes))
	for _, note := range r.notes {
		if note.UserID == input.UserID {
			items = append(items, note)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := len(items)
	if input.Offset >= total {
		return appreview.ListNotesResult{Items: []appreview.Note{}, Total: total}, nil
	}
	end := input.Offset + input.Limit
	if end > total {
		end = total
	}
	return appreview.ListNotesResult{Items: append([]appreview.Note(nil), items[input.Offset:end]...), Total: total}, nil
}
