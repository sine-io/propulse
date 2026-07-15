package gormrepo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	appreview "github.com/sine-io/propulse/internal/application/review"
	domainreview "github.com/sine-io/propulse/internal/domain/review"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReviewRepository 持久化复盘/看房笔记（WATCH-006.1 / #58）。
type ReviewRepository struct {
	db *gorm.DB
}

func NewReviewRepository(db *gorm.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) CreateNote(ctx context.Context, input appreview.CreateNoteInput) (appreview.Note, error) {
	if err := input.Validate(); err != nil {
		return appreview.Note{}, err
	}
	content, err := domainreview.NormalizeContent(input.Content)
	if err != nil {
		return appreview.Note{}, err
	}

	model := ReviewNoteModel{
		ID:             uuid.NewString(),
		UserID:         strings.TrimSpace(input.UserID),
		NeighborhoodID: input.NeighborhoodID,
		Kind:           string(input.Kind),
		WeekStartDate:  input.WeekStartDate,
		Content:        content,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return appreview.Note{}, err
	}
	return reviewNoteFromModel(model), nil
}

func (r *ReviewRepository) NeighborhoodExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&NeighborhoodModel{}).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

func (r *ReviewRepository) FindNote(ctx context.Context, userID string, id string) (appreview.Note, error) {
	var model ReviewNoteModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appreview.Note{}, appreview.ErrNoteNotFound
		}
		return appreview.Note{}, err
	}
	return reviewNoteFromModel(model), nil
}

func (r *ReviewRepository) UpdateNoteContent(ctx context.Context, userID string, id string, rawContent string) (appreview.Note, error) {
	content, err := domainreview.NormalizeContent(rawContent)
	if err != nil {
		return appreview.Note{}, err
	}

	var model ReviewNoteModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"content":    content,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return appreview.Note{}, result.Error
	}
	if result.RowsAffected == 0 {
		return appreview.Note{}, appreview.ErrNoteNotFound
	}
	return reviewNoteFromModel(model), nil
}

func (r *ReviewRepository) ListNotes(ctx context.Context, input appreview.ListNotesInput) (appreview.ListNotesResult, error) {
	if strings.TrimSpace(input.UserID) == "" || input.Limit < 1 || input.Limit > 100 || input.Offset < 0 {
		return appreview.ListNotesResult{}, appreview.ErrInvalidNote
	}

	query := r.db.WithContext(ctx).
		Model(&ReviewNoteModel{}).
		Where("user_id = ?", input.UserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return appreview.ListNotesResult{}, err
	}

	var models []ReviewNoteModel
	err := query.
		Order("created_at DESC, id DESC").
		Limit(input.Limit).
		Offset(input.Offset).
		Find(&models).Error
	if err != nil {
		return appreview.ListNotesResult{}, err
	}

	notes := make([]appreview.Note, 0, len(models))
	for _, model := range models {
		notes = append(notes, reviewNoteFromModel(model))
	}
	return appreview.ListNotesResult{Items: notes, Total: int(total)}, nil
}

func reviewNoteFromModel(model ReviewNoteModel) appreview.Note {
	return appreview.Note{
		ID:             model.ID,
		UserID:         model.UserID,
		NeighborhoodID: model.NeighborhoodID,
		Kind:           appreview.Kind(model.Kind),
		WeekStartDate:  model.WeekStartDate,
		Content:        model.Content,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
