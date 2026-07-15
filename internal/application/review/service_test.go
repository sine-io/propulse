package review

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const testUserID = "stable-user"

func TestServiceCreateNoteNormalizesAndUsesConfiguredUser(t *testing.T) {
	week := time.Date(2026, 7, 13, 15, 30, 0, 0, time.FixedZone("test", 8*60*60))
	neighborhoodID := "11111111-1111-4111-8111-111111111111"
	repo := &reviewRepositoryStub{neighborhoodExists: true}
	service := NewService(repo, "  "+testUserID+"  ")

	_, err := service.CreateNote(context.Background(), CreateNoteCommand{
		NeighborhoodID: &neighborhoodID,
		Kind:           KindReview,
		WeekStartDate:  &week,
		Content:        " \n  本周复盘  \t",
	})
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if repo.createInput.UserID != testUserID || repo.createInput.Content != "本周复盘" {
		t.Fatalf("CreateNote input = %#v", repo.createInput)
	}
	if repo.createInput.NeighborhoodID == nil || *repo.createInput.NeighborhoodID != neighborhoodID {
		t.Fatalf("NeighborhoodID = %#v", repo.createInput.NeighborhoodID)
	}
	if got := repo.createInput.WeekStartDate; got == nil || got.Location() != time.UTC || got.Hour() != 0 || got.Day() != 13 {
		t.Fatalf("WeekStartDate = %#v, want UTC date", got)
	}
}

func TestServiceCreateNoteValidation(t *testing.T) {
	missingNeighborhoodID := "22222222-2222-4222-8222-222222222222"
	tests := []struct {
		name    string
		service *Service
		command CreateNoteCommand
		wantErr error
	}{
		{name: "missing configured user", service: NewService(&reviewRepositoryStub{}, ""), command: CreateNoteCommand{Kind: KindReview, Content: "x"}, wantErr: ErrInvalidNote},
		{name: "invalid kind", service: NewService(&reviewRepositoryStub{}, testUserID), command: CreateNoteCommand{Kind: Kind("other"), Content: "x"}, wantErr: ErrInvalidNote},
		{name: "whitespace content", service: NewService(&reviewRepositoryStub{}, testUserID), command: CreateNoteCommand{Kind: KindReview, Content: " \n"}, wantErr: ErrInvalidNote},
		{name: "invalid neighborhood id", service: NewService(&reviewRepositoryStub{}, testUserID), command: CreateNoteCommand{Kind: KindReview, Content: "x", NeighborhoodID: stringPointer("bad")}, wantErr: ErrInvalidNeighborhoodID},
		{name: "missing neighborhood", service: NewService(&reviewRepositoryStub{neighborhoodExists: false}, testUserID), command: CreateNoteCommand{Kind: KindReview, Content: "x", NeighborhoodID: &missingNeighborhoodID}, wantErr: ErrNeighborhoodNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.service.CreateNote(context.Background(), tt.command)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateNote() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceUpdateAndGetUseConfiguredUser(t *testing.T) {
	noteID := "33333333-3333-4333-8333-333333333333"
	repo := &reviewRepositoryStub{}
	service := NewService(repo, testUserID)

	if _, err := service.UpdateNote(context.Background(), UpdateNoteCommand{ID: strings.ToUpper(noteID), Content: "  更新正文  "}); err != nil {
		t.Fatalf("UpdateNote() error = %v", err)
	}
	if repo.updateUserID != testUserID || repo.updateID != noteID || repo.updateContent != "更新正文" {
		t.Fatalf("update arguments = user %q id %q content %q", repo.updateUserID, repo.updateID, repo.updateContent)
	}
	if _, err := service.GetNote(context.Background(), GetNoteQuery{ID: noteID}); err != nil {
		t.Fatalf("GetNote() error = %v", err)
	}
	if repo.findUserID != testUserID || repo.findID != noteID {
		t.Fatalf("find arguments = user %q id %q", repo.findUserID, repo.findID)
	}
}

func TestServiceUpdateAndGetRejectInvalidInput(t *testing.T) {
	service := NewService(&reviewRepositoryStub{}, testUserID)
	validID := "44444444-4444-4444-8444-444444444444"
	for _, id := range []string{
		"bad",
		" " + validID,
		strings.ReplaceAll(validID, "-", ""),
		"{" + validID + "}",
	} {
		if _, err := service.UpdateNote(context.Background(), UpdateNoteCommand{ID: id, Content: "x"}); !errors.Is(err, ErrInvalidNoteID) {
			t.Fatalf("UpdateNote(ID %q) error = %v, want ErrInvalidNoteID", id, err)
		}
		if _, err := service.GetNote(context.Background(), GetNoteQuery{ID: id}); !errors.Is(err, ErrInvalidNoteID) {
			t.Fatalf("GetNote(ID %q) error = %v, want ErrInvalidNoteID", id, err)
		}
	}
	if _, err := service.UpdateNote(context.Background(), UpdateNoteCommand{ID: validID, Content: " \t"}); !errors.Is(err, ErrInvalidNote) {
		t.Fatalf("UpdateNote() error = %v, want ErrInvalidNote", err)
	}
}

func TestServiceListNotesNormalizesPaginationAndUsesConfiguredUser(t *testing.T) {
	tests := []struct {
		name         string
		query        ListNotesQuery
		wantPage     int
		wantPageSize int
		wantOffset   int
	}{
		{name: "defaults", query: ListNotesQuery{}, wantPage: 1, wantPageSize: 20, wantOffset: 0},
		{name: "maximum", query: ListNotesQuery{Page: 3, PageSize: 101}, wantPage: 3, wantPageSize: 100, wantOffset: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &reviewRepositoryStub{listResult: ListNotesResult{Total: 7}}
			page, err := NewService(repo, testUserID).ListNotes(context.Background(), tt.query)
			if err != nil {
				t.Fatalf("ListNotes() error = %v", err)
			}
			if page.Page != tt.wantPage || page.PageSize != tt.wantPageSize || page.Total != 7 || page.Items == nil {
				t.Fatalf("page = %#v", page)
			}
			if repo.listInput.UserID != testUserID || repo.listInput.Limit != tt.wantPageSize || repo.listInput.Offset != tt.wantOffset {
				t.Fatalf("list input = %#v", repo.listInput)
			}
		})
	}
}

type reviewRepositoryStub struct {
	neighborhoodExists bool
	createInput        CreateNoteInput
	findUserID         string
	findID             string
	updateUserID       string
	updateID           string
	updateContent      string
	listInput          ListNotesInput
	listResult         ListNotesResult
}

func (r *reviewRepositoryStub) CreateNote(_ context.Context, input CreateNoteInput) (Note, error) {
	r.createInput = input
	return Note{}, nil
}

func (r *reviewRepositoryStub) NeighborhoodExists(context.Context, string) (bool, error) {
	return r.neighborhoodExists, nil
}

func (r *reviewRepositoryStub) FindNote(_ context.Context, userID string, id string) (Note, error) {
	r.findUserID = userID
	r.findID = id
	return Note{}, nil
}

func (r *reviewRepositoryStub) UpdateNoteContent(_ context.Context, userID string, id string, content string) (Note, error) {
	r.updateUserID = userID
	r.updateID = id
	r.updateContent = content
	return Note{}, nil
}

func (r *reviewRepositoryStub) ListNotes(_ context.Context, input ListNotesInput) (ListNotesResult, error) {
	r.listInput = input
	return r.listResult, nil
}
