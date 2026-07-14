// Package review 定义复盘记录与看房笔记的领域模型与持久化端口（WATCH-006.1 / #58）。
// 本子任务只建立数据模型与基础存取能力；受保护的 CRUD API 由 #59 负责。
package review

import (
	"context"
	"errors"
	"time"
)

// Kind 区分复盘记录与看房笔记。
type Kind string

const (
	KindReview      Kind = "review"
	KindViewingNote Kind = "viewing_note"
)

const maxContentLength = 8000

// ErrInvalidNote 表示笔记内容或类型不满足约束。
var ErrInvalidNote = errors.New("invalid review note")

// ErrNoteNotFound 表示按 ID 未找到笔记。
var ErrNoteNotFound = errors.New("review note not found")

// Note 是复盘/看房笔记实体，关联稳定用户身份与可选小区、周次。
type Note struct {
	ID             string
	UserID         string
	NeighborhoodID *string
	Kind           Kind
	WeekStartDate  *time.Time
	Content        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateNoteInput 是创建笔记的入参（ID 由 repository 生成）。
type CreateNoteInput struct {
	UserID         string
	NeighborhoodID *string
	Kind           Kind
	WeekStartDate  *time.Time
	Content        string
}

// Validate 校验创建入参：用户与内容非空、类型合法、内容长度受限。
func (input CreateNoteInput) Validate() error {
	if input.UserID == "" {
		return ErrInvalidNote
	}
	if input.Kind != KindReview && input.Kind != KindViewingNote {
		return ErrInvalidNote
	}
	if l := len(input.Content); l == 0 || l > maxContentLength {
		return ErrInvalidNote
	}
	return nil
}

// Repository 定义复盘笔记的持久化端口。
type Repository interface {
	CreateNote(ctx context.Context, input CreateNoteInput) (Note, error)
	ListNotesByUser(ctx context.Context, userID string) ([]Note, error)
}
