package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	appreview "github.com/sine-io/propulse/internal/application/review"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
	"gorm.io/gorm"
)

func TestReviewRepositoryPostgresCRUDOwnershipAndPagination(t *testing.T) {
	repo, db := openReviewRepositoryTestTransaction(t)
	ctx := context.Background()
	userID := "review-test-" + uuid.NewString()
	otherUserID := userID + "-other"
	firstNeighborhoodID := insertReviewTestNeighborhood(t, db, "review-neighborhood-a")
	secondNeighborhoodID := insertReviewTestNeighborhood(t, db, "review-neighborhood-b")
	firstWeek := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	secondWeek := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	created, err := repo.CreateNote(ctx, appreview.CreateNoteInput{
		UserID:         userID,
		NeighborhoodID: &firstNeighborhoodID,
		Kind:           appreview.KindReview,
		WeekStartDate:  &firstWeek,
		Content:        "  first review  ",
	})
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if created.ID == "" || created.Content != "first review" || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created note = %#v", created)
	}

	found, err := repo.FindNote(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("FindNote() error = %v", err)
	}
	if found.ID != created.ID || found.UserID != userID || found.NeighborhoodID == nil || *found.NeighborhoodID != firstNeighborhoodID {
		t.Fatalf("found note = %#v", found)
	}

	time.Sleep(2 * time.Millisecond)
	updated, err := repo.UpdateNoteContent(ctx, userID, created.ID, " updated review ")
	if err != nil {
		t.Fatalf("UpdateNoteContent() error = %v", err)
	}
	if updated.Content != "updated review" || !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("updated note = %#v, created UpdatedAt = %v", updated, created.UpdatedAt)
	}
	refreshed, err := repo.FindNote(ctx, userID, created.ID)
	if err != nil || refreshed.Content != updated.Content || !refreshed.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("refreshed note = %#v, error = %v", refreshed, err)
	}

	if _, err := repo.FindNote(ctx, otherUserID, created.ID); !errors.Is(err, appreview.ErrNoteNotFound) {
		t.Fatalf("FindNote(other user) error = %v, want ErrNoteNotFound", err)
	}
	if _, err := repo.UpdateNoteContent(ctx, otherUserID, created.ID, "must not persist"); !errors.Is(err, appreview.ErrNoteNotFound) {
		t.Fatalf("UpdateNoteContent(other user) error = %v, want ErrNoteNotFound", err)
	}
	unchanged, err := repo.FindNote(ctx, userID, created.ID)
	if err != nil || unchanged.Content != "updated review" {
		t.Fatalf("note after rejected update = %#v, error = %v", unchanged, err)
	}

	createdIDs := []string{created.ID}
	for index, input := range []appreview.CreateNoteInput{
		{UserID: userID, NeighborhoodID: &secondNeighborhoodID, Kind: appreview.KindViewingNote, WeekStartDate: &secondWeek, Content: "second context"},
		{UserID: userID, NeighborhoodID: &firstNeighborhoodID, Kind: appreview.KindReview, WeekStartDate: &secondWeek, Content: "third context"},
		{UserID: userID, Kind: appreview.KindReview, Content: "fourth context"},
		{UserID: userID, Kind: appreview.KindViewingNote, Content: "fifth context"},
	} {
		note, err := repo.CreateNote(ctx, input)
		if err != nil {
			t.Fatalf("CreateNote(%d) error = %v", index, err)
		}
		createdIDs = append(createdIDs, note.ID)
	}
	if _, err := repo.CreateNote(ctx, appreview.CreateNoteInput{UserID: otherUserID, Kind: appreview.KindReview, Content: "private other-user note"}); err != nil {
		t.Fatalf("CreateNote(other user) error = %v", err)
	}

	stableTimestamp := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	if err := db.Model(&ReviewNoteModel{}).Where("id IN ?", createdIDs).Update("created_at", stableTimestamp).Error; err != nil {
		t.Fatalf("set stable created_at: %v", err)
	}
	slices.Sort(createdIDs)
	slices.Reverse(createdIDs)

	firstPage, err := repo.ListNotes(ctx, appreview.ListNotesInput{UserID: userID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListNotes(first page) error = %v", err)
	}
	secondPage, err := repo.ListNotes(ctx, appreview.ListNotesInput{UserID: userID, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListNotes(second page) error = %v", err)
	}
	thirdPage, err := repo.ListNotes(ctx, appreview.ListNotesInput{UserID: userID, Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("ListNotes(third page) error = %v", err)
	}
	if firstPage.Total != 5 || secondPage.Total != 5 || thirdPage.Total != 5 {
		t.Fatalf("page totals = %d, %d, %d", firstPage.Total, secondPage.Total, thirdPage.Total)
	}
	gotIDs := append(reviewNoteIDs(firstPage.Items), reviewNoteIDs(secondPage.Items)...)
	gotIDs = append(gotIDs, reviewNoteIDs(thirdPage.Items)...)
	if fmt.Sprint(gotIDs) != fmt.Sprint(createdIDs) {
		t.Fatalf("stable paginated IDs = %v, want %v", gotIDs, createdIDs)
	}
	if len(slices.Compact(append([]string(nil), gotIDs...))) != len(gotIDs) {
		t.Fatalf("pagination returned duplicate IDs: %v", gotIDs)
	}

	contexts := make(map[string]appreview.Note, len(gotIDs))
	for _, page := range []appreview.ListNotesResult{firstPage, secondPage, thirdPage} {
		for _, note := range page.Items {
			contexts[note.Content] = note
		}
	}
	if contexts["second context"].NeighborhoodID == nil || *contexts["second context"].NeighborhoodID != secondNeighborhoodID ||
		contexts["third context"].WeekStartDate == nil || !contexts["third context"].WeekStartDate.Equal(secondWeek) {
		t.Fatalf("distinct historical contexts were not preserved: %#v", contexts)
	}
}

func TestReviewRepositoryRejectsInvalidInputBeforeDatabaseAccess(t *testing.T) {
	repo := NewReviewRepository(nil)
	if _, err := repo.CreateNote(context.Background(), appreview.CreateNoteInput{Kind: appreview.KindReview, Content: "x"}); err == nil {
		t.Fatal("CreateNote() error = nil, want validation error")
	}
	if _, err := repo.UpdateNoteContent(context.Background(), "user", "id", " \n"); err == nil {
		t.Fatal("UpdateNoteContent() error = nil, want validation error")
	}
	if _, err := repo.ListNotes(context.Background(), appreview.ListNotesInput{UserID: "user"}); err == nil {
		t.Fatal("ListNotes() error = nil, want validation error")
	}
}

func openReviewRepositoryTestTransaction(t *testing.T) (*ReviewRepository, *gorm.DB) {
	t.Helper()
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
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("Begin() error = %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return NewReviewRepository(tx), tx
}

func insertReviewTestNeighborhood(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	id := uuid.NewString()
	if err := db.Create(&NeighborhoodModel{ID: id, Name: name, Area: "test-area"}).Error; err != nil {
		t.Fatalf("create neighborhood: %v", err)
	}
	return id
}

func reviewNoteIDs(notes []appreview.Note) []string {
	ids := make([]string, 0, len(notes))
	for _, note := range notes {
		ids = append(ids, note.ID)
	}
	return ids
}
