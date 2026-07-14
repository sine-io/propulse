package gormrepo

import (
	"context"
	"os"
	"testing"

	appreview "github.com/sine-io/propulse/internal/application/review"
	"github.com/sine-io/propulse/internal/application/user"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

func TestReviewRepositoryPersistsAndListsNotes(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("Run(up) error = %v", err)
	}

	db, sqlDB, err := Open(databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := NewReviewRepository(db)

	created, err := repo.CreateNote(ctx, appreview.CreateNoteInput{
		UserID:  user.SingleUserID,
		Kind:    appreview.KindReview,
		Content: "本周复盘：目标小区降价房源增加，继续观察。",
	})
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created note missing generated fields: %#v", created)
	}

	notes, err := repo.ListNotesByUser(ctx, user.SingleUserID)
	if err != nil {
		t.Fatalf("ListNotesByUser() error = %v", err)
	}
	found := false
	for _, note := range notes {
		if note.ID == created.ID && note.Content == created.Content {
			found = true
		}
	}
	if !found {
		t.Fatalf("created note not returned by ListNotesByUser: %#v", notes)
	}
}

func TestReviewRepositoryRejectsInvalidNote(t *testing.T) {
	// 无需数据库：校验在写库前发生。
	repo := NewReviewRepository(nil)
	_, err := repo.CreateNote(context.Background(), appreview.CreateNoteInput{
		UserID:  "",
		Kind:    appreview.KindReview,
		Content: "x",
	})
	if err == nil {
		t.Fatal("CreateNote() = nil error, want validation error")
	}
}
