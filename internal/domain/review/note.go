package review

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type Kind string

const (
	KindReview      Kind = "review"
	KindViewingNote Kind = "viewing_note"
	MaxContentRunes      = 8000
)

var ErrInvalidNote = errors.New("invalid review note")

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

func IsValidKind(kind Kind) bool {
	return kind == KindReview || kind == KindViewingNote
}

func NormalizeContent(content string) (string, error) {
	normalized := strings.TrimSpace(content)
	length := utf8.RuneCountInString(normalized)
	if length == 0 || length > MaxContentRunes {
		return "", ErrInvalidNote
	}
	return normalized, nil
}
