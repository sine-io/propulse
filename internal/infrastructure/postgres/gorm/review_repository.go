package gormrepo

import (
	"context"

	"github.com/google/uuid"
	appreview "github.com/sine-io/propulse/internal/application/review"
	"gorm.io/gorm"
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

	model := ReviewNoteModel{
		ID:             uuid.NewString(),
		UserID:         input.UserID,
		NeighborhoodID: input.NeighborhoodID,
		Kind:           string(input.Kind),
		WeekStartDate:  input.WeekStartDate,
		Content:        input.Content,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return appreview.Note{}, err
	}
	return reviewNoteFromModel(model), nil
}

func (r *ReviewRepository) ListNotesByUser(ctx context.Context, userID string) ([]appreview.Note, error) {
	var models []ReviewNoteModel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	notes := make([]appreview.Note, 0, len(models))
	for _, model := range models {
		notes = append(notes, reviewNoteFromModel(model))
	}
	return notes, nil
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
